package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/drone/drone-go/drone"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/lithammer/shortuuid/v3"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/oauth2"
	"gopkg.in/ini.v1"
)

var (
	TOKEN         = ""
	HOST          = ""
	DATADIR       = filepath.Join(xdg.DataHome, "buildrone")
	STORAGE       = filepath.Join(DATADIR, "buildfiles")
	CONFIG        = filepath.Join(xdg.ConfigHome, "buildrone", "config.ini")
	DIRPERM       = 0700
	TOKEN_PERIOD  = 40 // Refresh token expiry in days. Essentially the longest time without a build before you need to make a new token.
	BUILDSPERPAGE = 6
	DEBUG         = false
	SERVE         = "0.0.0.0"
	PORT          = 8062
	MAXAGE        = ""
	MAXAGEDELTA   maxAgeDelta
	LOGIPS        = false
)

func parseNum(str string, d string) int {
	if !strings.Contains(str, d) {
		return 0
	}
	s := strings.Split(str, d)[0]
	num := ""
	for _, c := range s {
		_, err := strconv.Atoi(string(c))
		if err == nil {
			num += string(c)
		}
		if err != nil {
			num = ""
		}
	}
	n, err := strconv.Atoi(num)
	if err != nil {
		return 0
	}
	return n
}

type maxAgeDelta = func(t time.Time) bool

func parseMaxAge(maxage string) (f maxAgeDelta) {
	// h = hours, d = days, y = years
	years := parseNum(maxage, "y")
	days := parseNum(maxage, "d")
	hours := parseNum(maxage, "h")
	minutes := parseNum(maxage, "m")
	if maxage == "" || (years == 0 && days == 0 && hours == 0 && minutes == 0) {
		f = func(t time.Time) bool { return false }
	} else {
		f = func(t time.Time) bool {
			return time.Now().After(t.Add(time.Duration(365*24*years+24*days+hours)*time.Hour + time.Duration(minutes)*time.Minute))
		}
	}
	return
}

// Tags are used for non-buildrone builds to tell apps that the specific update is ready, along with providing basic info. App is meant to do the heavylifting in terms of actually acquiring the build.
// Created on-the-fly by upload.py
type Tag struct {
	Ready       bool   `json:"ready"`             // Whether or not build on this tag has completed.
	Version     string `json:"version,omitempty"` // Version/Commit
	ReleaseDate Time   `json:"date"`
}

type Build struct {
	ID          int64
	Branch      string
	Name        string // commit line
	Date        time.Time
	DateChanged time.Time
	Files       string
	Link        string
	Message     string
	Tags        map[string]Tag
}

type Repo struct {
	Namespace, Name, Link, LatestBuild, LatestNonEmptyBuild string
	Builds                                                  map[string]Build // map[commitHash]
	Branches                                                []string
	Secret                                                  string
}

type appContext struct {
	config   *ini.File
	client   drone.Client
	storage  map[string]Repo
	fs       http.FileSystem
	Username string
	Password string
	logTo    string
}

type RepoDTO struct {
	Namespace      string // `json:"namespace"`
	Name           string // `json:"name"`
	BuildPageCount uint   // `json:"builds"`
	Builds         map[string]BuildDTO
	LatestCommit   string
	LatestPush     BuildDTO
	Secret         bool
	Branches       []string
}

type BuildDTO struct {
	ID      int64     // `json:"id"`
	Name    string    // `json:"name"`
	Date    time.Time // `json:"date"`
	Files   []FileDTO // `json:"files"`
	Link    string    // `json:"link"`
	Message string
	Branch  string // `json:"branch"`
	Tags    map[string]Tag
}

type FileDTO struct {
	Name string
	Size string
}

// storage is map["namespace/name"]Repo

// Get human-readable file size from f.Size() result.
// https://programming.guide/go/formatting-byte-size-to-human-readable-format.html
func fileSize(l int64) string {
	const unit = 1000
	if l < unit {
		return fmt.Sprintf("%dB", l)
	}
	div, exp := int64(unit), 0
	for n := l / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(l)/float64(div), "KMGTPE"[exp])
}

func (app *appContext) loadRepos() (err error) {
	dRepos, err := app.client.RepoList()
	if err != nil {
		return
	}
	for _, dRepo := range dRepos {
		if !dRepo.Active {
			continue
		}
		id := dRepo.Namespace + "/" + dRepo.Name
		if _, ok := app.storage[id]; !ok {
			newRepo := Repo{
				Namespace: dRepo.Namespace,
				Name:      dRepo.Name,
				Link:      dRepo.Link,
				Secret:    "",
			}
			newRepo.Builds = map[string]Build{}
			app.storage[id] = newRepo
		}
	}
	return
}

type NewKeyReqDTO struct {
	NewSecret bool
}

type NewKeyRespDTO struct {
	Key string
}

func (app *appContext) loadBuilds(bl map[string]Build, ns, name string) (builds map[string]Build, branches []string, latestBuild string, latestNonEmptyBuild string, err error) {
	dBuildList, err := app.client.BuildList(ns, name, drone.ListOptions{Page: 1, Size: 500})
	if err != nil {
		return
	}
	latestTime := time.Time{}
	latestNETime := time.Time{}
	builds = map[string]Build{}
	for _, dBuild := range dBuildList {
		commit := dBuild.After
		build := Build{
			ID:     dBuild.ID,
			Name:   strings.Split(dBuild.Message, "\n")[0],
			Date:   time.Unix(dBuild.Updated, 0),
			Link:   dBuild.Link,
			Branch: dBuild.Target,
		}
		if build.Branch == "" {
			build.Branch = dBuild.Source
		}
		if build.Branch != "" {
			exists := false
			for _, v := range branches {
				if v == build.Branch {
					exists = true
					break
				}
			}
			if !exists {
				branches = append(branches, build.Branch)
			}
		}
		if b, ok := bl[commit]; ok {
			build.Files = b.Files
			build.DateChanged = b.DateChanged
			build.Tags = b.Tags
			t := time.Time{}
			if build.DateChanged == t {
				build.DateChanged = build.Date
			}
			if build.Files != "" && MAXAGEDELTA(build.DateChanged) {
				log.Printf("Removing old files for commit %s", commit)
				os.RemoveAll(filepath.Join(STORAGE, build.Files))
				build.Files = ""
			}
		}
		if build.Date.After(latestTime) {
			latestTime = build.Date
			latestBuild = commit
		}
		if build.Date.After(latestNETime) && build.Files != "" {
			if d, err := os.ReadDir(filepath.Join(STORAGE, build.Files)); err == nil && len(d) != 0 {
				latestNETime = build.Date
				latestNonEmptyBuild = commit
			}
		}
		builds[commit] = build
	}
	return
}

func (app *appContext) store() error {
	path := filepath.Join(DATADIR, "storage.gob")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := gob.NewEncoder(file)
	return enc.Encode(app.storage)
}

func (app *appContext) read() error {
	path := filepath.Join(DATADIR, "storage.gob")
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	dec := gob.NewDecoder(file)
	return dec.Decode(&app.storage)
}

func setKey(config *ini.File, key, value, comment string) {
	config.Section("").Key(key).SetValue(value)
	config.Section("").Key(key).Comment = comment
}

func (app *appContext) loadAllBuilds() {
	for n, repo := range app.storage {
		log.Printf("Loading builds for %s/%s", repo.Namespace, repo.Name)
		builds, branches, latest, latestNE, err := app.loadBuilds(repo.Builds, repo.Namespace, repo.Name)
		if err == nil {
			repo.LatestBuild = latest
			repo.LatestNonEmptyBuild = latestNE
			repo.Builds = builds
			repo.Branches = branches
			app.storage[n] = repo
		}
	}
	app.store()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "password" {
		fmt.Print("Enter your new password: ")
		password, err := terminal.ReadPassword(0)
		if err != nil {
			panic(err)
		}
		hash, err := hash(string(password))
		if string(password) != "" && err == nil {
			fmt.Printf("\nAdd this to your config:\npassword_hash = %s\nusername = <your username>\n\nMake sure to keep the hash a secret.\n", hash)
			os.Exit(0)
		}
	}

	flag.StringVar(&CONFIG, "config", CONFIG, "location of config file (ini)")
	flag.StringVar(&DATADIR, "data", DATADIR, "location of stored database and build files")
	flag.StringVar(&SERVE, "host", SERVE, "address to host app on")
	flag.IntVar(&PORT, "port", PORT, "port to host app on")
	flag.BoolVar(&DEBUG, "debug", DEBUG, "use debug mode")
	flag.StringVar(&MAXAGE, "maxage", MAXAGE, "Delete files from commits once they are this old. example: 1y30d2h (m = minutes, h = hours, d = days, y = years).")

	flag.Parse()
	STORAGE = filepath.Join(DATADIR, "buildfiles")

	if _, err := os.Stat(DATADIR); os.IsNotExist(err) {
		log.Printf("Creating data directory at \"%s\"", DATADIR)
		os.MkdirAll(STORAGE, os.FileMode(DIRPERM))
	}

	if _, err := os.Stat(CONFIG); os.IsNotExist(err) {
		dir, _ := filepath.Split(CONFIG)
		os.MkdirAll(dir, os.FileMode(DIRPERM))
		f, err := os.Create(CONFIG)
		if err != nil {
			log.Fatalf("Failed to create new config at \"%s\"", CONFIG)
		}
		f.Close()
		tempConfig, err := ini.Load(CONFIG)
		if err != nil {
			log.Fatalf("Failed to create new config at \"%s\"", CONFIG)
		}
		setKey(tempConfig, "drone_host", "https://drone.url", "Drone URL.")
		setKey(tempConfig, "drone_apikey", "", "Drone API key. Can be generated in user settings.")
		setKey(tempConfig, "token_period", strconv.Itoa(TOKEN_PERIOD), "Build token expiry in days. After generating a build key, you will have this long before you need to regenerate.")
		setKey(tempConfig, "max_file_age", "1y", "Maximum age of files on a commit. example: 1y30d2h (y = years, d = days, h = hours, m = minutes).")
		setKey(tempConfig, "username", "your username", "Web UI username.")
		setKey(tempConfig, "password_hash", "", "Web UI password hash. Generate by running \"buildrone password\".")
		setKey(tempConfig, "user_log", "", "URL to log ips to, IP will be appended. Recommended for use with github.com/hrfee/ipcount. Leave blank to disable.")
		err = tempConfig.SaveTo(CONFIG)
		if err != nil {
			log.Fatalf("Failed to save template config at \"%s\"", CONFIG)
		}
		log.Fatalf("Template config created at \"%s\".\nFill it in, and restart.", CONFIG)
	}

	app := &appContext{}
	var err error
	app.config, err = ini.Load(CONFIG)
	if err != nil {
		log.Fatalf("Failed to load config: %s", err)
	}

	if maxage := app.config.Section("").Key("max_file_age").MustString(""); maxage != "" {
		MAXAGE = maxage
	}

	MAXAGEDELTA = parseMaxAge(MAXAGE)

	app.storage = map[string]Repo{}
	TOKEN_PERIOD = app.config.Section("").Key("token_period").MustInt(TOKEN_PERIOD)
	os.Setenv("BUILDRONE_SECRET", app.config.Section("").Key("secret_key").String())
	os.Setenv("BUILDRONE_WEBSECRET", shortuuid.New())
	TOKEN = app.config.Section("").Key("drone_apikey").String()
	HOST = app.config.Section("").Key("drone_host").String()
	config := new(oauth2.Config)
	auth := config.Client(
		oauth2.NoContext,
		&oauth2.Token{
			AccessToken: TOKEN,
		},
	)

	ipPath := app.config.Section("").Key("user_log").String()
	if ipPath != "" {
		LOGIPS = true
		app.logTo = ipPath
	}

	app.client = drone.NewClient(HOST, auth)
	app.read()
	log.Printf("Loading httpFilesystem")
	app.fs = http.Dir(STORAGE)
	app.loadRepos()
	log.Printf("Loading repos & builds")
	app.loadAllBuilds()
	log.Printf("Setting up router")
	if DEBUG {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	executable, _ := os.Executable()
	router.LoadHTMLGlob(filepath.Join(filepath.Dir(executable), "templates/*"))
	router.Use(static.Serve("/", static.LocalFile(filepath.Join(filepath.Dir(executable), "static"), false)))
	router.GET("/repo/:namespace/:name/token", app.getBuildToken)
	router.GET("/repo/:namespace/:name/tag/:build/:tag", app.GetTag)
	router.GET("/repo/:namespace/:name/build/:build/:file", app.getFile)
	router.GET("/repo/:namespace/:name/latest/file/:search", app.findLatest)
	router.GET("/repo/:namespace/:name/latest", app.LatestCommit)
	router.GET("/repo/:namespace/:name/build/:build", app.getBuild)
	router.GET("/repo/:namespace/:name/builds/:page", app.getBuilds)
	router.GET("/repo/:namespace/:name", app.getRepo)
	router.GET("/", func(gc *gin.Context) {
		gc.HTML(200, "admin.html", gin.H{})
	})
	router.GET("/view/:namespace/:name", func(gc *gin.Context) {
		ns := gc.Param("namespace")
		name := gc.Param("name")
		_, ok := app.storage[ns+"/"+name]
		if !ok {
			end(400, fmt.Sprintf("Repo not found: %s/%s", ns, name), gc)
			return
		}
		gc.HTML(200, "repo.html", gin.H{
			"namespace": ns,
			"name":      name,
			"repoLink":  app.storage[ns+"/"+name].Link,
		})
	})
	router.GET("/token", app.getWebToken)
	adminAPI := router.Group("/", app.webAuth())
	adminAPI.GET("/repos", app.getRepos)
	adminAPI.POST("/repo/:namespace/:name/key", app.NewKey)
	handler := func(gc *gin.Context) {
		query := gc.Param("query")
		if query == "add" {
			app.addFiles(gc)
		} else if query == "tag" {
			app.SetTag(gc)
		}
	}
	buildAPI := router.Group("/", app.buildAuth())
	buildAPI.POST("/repo/:namespace/:name/commit/:commit/:query", handler)
	buildAPI.POST("/repo/:namespace/:name/commit/:commit/:query/:tag", handler)
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", SERVE, PORT),
		Handler: router,
	}
	go func() {
		for {
			time.Sleep(5 * 60 * time.Second)
			log.Println("Reloading repos")
			app.loadRepos()
			app.loadAllBuilds()
		}
	}()
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalln("Failed to serve:", err)
	}
}
