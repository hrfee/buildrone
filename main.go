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
)

type Build struct {
	ID    int64
	Name  string // commit line
	Date  time.Time
	Files string
	Link  string
}

type Repo struct {
	Namespace, Name, Link string
	Builds                map[string]Build // map[commitHash]
	Secret                string
}

type appContext struct {
	config   *ini.File
	client   drone.Client
	storage  map[string]Repo
	fs       http.FileSystem
	Username string
	Password string
}

type RepoDTO struct {
	Namespace      string // `json:"namespace"`
	Name           string // `json:"name"`
	BuildPageCount uint   // `json:"builds"`
	Builds         map[string]BuildDTO
	LatestCommit   string
	LatestPush     BuildDTO
	Secret         bool
}

type BuildDTO struct {
	ID    int64     // `json:"id"`
	Name  string    // `json:"name"`
	Date  time.Time // `json:"date"`
	Files []FileDTO // `json:"files"`
	Link  string    // `json:"link"`
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

func (app *appContext) loadBuilds(bl map[string]Build, ns, name string) (builds map[string]Build, err error) {
	dBuildList, err := app.client.BuildList(ns, name, drone.ListOptions{Page: 1, Size: 500})
	if err != nil {
		return
	}
	builds = map[string]Build{}
	for _, dBuild := range dBuildList {
		commit := dBuild.After
		build := Build{
			ID:   dBuild.ID,
			Name: dBuild.Title,
			Date: time.Unix(dBuild.Updated, 0),
			Link: dBuild.Link,
		}
		if b, ok := bl[commit]; ok {
			build.Files = b.Files
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
		builds, err := app.loadBuilds(repo.Builds, repo.Namespace, repo.Name)
		if err == nil {
			repo.Builds = builds
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
		setKey(tempConfig, "username", "your username", "Web UI username.")
		setKey(tempConfig, "password_hash", "", "Web UI password hash. Generate by running \"buildrone password\".")
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
	router.LoadHTMLGlob("templates/*")
	router.Use(static.Serve("/", static.LocalFile("static", false)))
	router.GET("/repo/:namespace/:name/token", app.getBuildToken)
	router.GET("/repo/:namespace/:name/build/:build/:file", app.getFile)
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
	buildAPI := router.Group("/", app.buildAuth())
	buildAPI.POST("/repo/:namespace/:name/add", app.addFiles)
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
	srv.ListenAndServe()

}
