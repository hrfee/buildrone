package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lithammer/shortuuid/v3"
)

func (app *appContext) NewKey(gc *gin.Context) {
	namespace := gc.Param("namespace")
	name := gc.Param("name")
	var req NewKeyReqDTO
	err := gc.BindJSON(&req)
	if err != nil {
		msg := fmt.Sprintf("Failed to bind request JSON: %s", err)
		log.Printf("%s/%s: %s", namespace, name, msg)
		end(400, msg, gc)
		return
	}
	err = app.loadRepos()
	if err != nil {
		end(500, fmt.Sprintf("Couldn't load repos: %s", err), gc)
		return
	}
	id := namespace + "/" + name
	repo, ok := app.storage[id]
	if !ok {
		end(400, fmt.Sprintf("Repo not found: %s/%s", namespace, name), gc)
		return
	}
	if req.NewSecret || repo.Secret == "" {
		log.Printf("%s/%s: Generating new secret (invalidating previous tokens)", namespace, name)
		repo.Secret = shortuuid.New()
	}
	if err != nil {
		end(500, fmt.Sprintf("Couldn't generate build token: %s", err), gc)
		return
	}
	app.storage[id] = repo
	err = app.store()
	if err != nil {
		end(500, fmt.Sprintf("Couldn't store data: %s", err), gc)
		return
	}
	log.Printf("%s/%s: Generating new key", namespace, name)
	_, key, err := newBuildToken(namespace, name, repo.Secret)
	if err != nil {
		end(500, fmt.Sprintf("Couldn't generate token: %s", err), gc)
		return
	}
	gc.JSON(200, NewKeyRespDTO{Key: key})
}

func (app *appContext) SetTag(gc *gin.Context) {
	var req Tag
	gc.BindJSON(&req)
	namespace := gc.Param("namespace")
	name := gc.Param("name")
	commit := gc.Param("commit")
	tagName := gc.Param("tag")
	repo, ok := app.storage[namespace+"/"+name]
	if !ok {
		end(400, fmt.Sprintf("Repository not found: %s/%s", namespace, name), gc)
		return
	}
	var err error
	repo.Builds, repo.Branches, repo.LatestBuild, repo.LatestNonEmptyBuild, err = app.loadBuilds(repo.Builds, namespace, name)
	if err != nil {
		end(500, "Couldn't load builds", gc)
		return
	}
	build, ok := repo.Builds[commit]
	if !ok {
		end(400, fmt.Sprintf("Commit not found: %s", commit), gc)
		return
	}
	tag, ok := build.Tags[tagName]
	if !ok {
		tag = req
	} else {
		if req.Version != "" && req.Version != tag.Version {
			tag.Version = req.Version
		}
		t := Time{}
		if req.ReleaseDate != t && req.ReleaseDate != tag.ReleaseDate {
			tag.ReleaseDate = req.ReleaseDate
		}
		tag.Ready = req.Ready
	}
	if build.Tags == nil {
		build.Tags = map[string]Tag{}
	}
	build.Tags[tagName] = tag
	repo.Builds[commit] = build
	app.storage[namespace+"/"+name] = repo
	app.store()
	end(200, "Tag stored", gc)
}

func (app *appContext) GetTag(gc *gin.Context) {
	namespace := gc.Param("namespace")
	name := gc.Param("name")
	commit := gc.Param("build")
	tagName := gc.Param("tag")
	repo, ok := app.storage[namespace+"/"+name]
	if !ok {
		end(400, fmt.Sprintf("Repository not found: %s/%s", namespace, name), gc)
		return
	}
	if commit == "latest" {
		commit = repo.LatestBuild
	}
	build, ok := repo.Builds[commit]
	if !ok {
		end(400, "Couldn't get build", gc)
		return
	}
	tag, ok := build.Tags[tagName]
	if !ok {
		tag = Tag{}
	}
	gc.JSON(200, tag)
}

func (app *appContext) addFiles(gc *gin.Context) {
	ns := gc.Param("namespace")
	name := gc.Param("name")
	commit := gc.Param("commit")

	form, err := gc.MultipartForm()
	if err != nil {
		end(400, fmt.Sprintf("Form error: %s", err), gc)
		log.Printf("%s/%s: Form error: %s", ns, name, err)
		return
	}
	files := form.File
	_, ok := app.storage[ns+"/"+name]
	if !ok {
		dRepo, err := app.client.Repo(ns, name)
		if err != nil {
			out := fmt.Sprintf("Repository not found: %s/%s", ns, name)
			end(400, out, gc)
			log.Println(out)
			return
		}
		newRepo := Repo{
			Namespace: ns,
			Name:      name,
			Link:      dRepo.Link,
			Secret:    shortuuid.New(),
		}
		newRepo.Builds = map[string]Build{}

		app.storage[ns+"/"+name] = newRepo
	}
	os.Mkdir(filepath.Join(STORAGE, ns), os.FileMode(DIRPERM))
	os.Mkdir(filepath.Join(STORAGE, ns, name), os.FileMode(DIRPERM))
	repo := app.storage[ns+"/"+name]
	repo.Builds, repo.Branches, repo.LatestBuild, repo.LatestNonEmptyBuild, err = app.loadBuilds(repo.Builds, ns, name)
	if err != nil {
		end(500, fmt.Sprintf("Couldn't get builds: %s", err), gc)
		return
	}
	commitDirectory := filepath.Join(ns, name, commit)
	err = os.MkdirAll(filepath.Join(STORAGE, commitDirectory), os.FileMode(DIRPERM))
	if err != nil {
		end(500, fmt.Sprintf("Couldn't create directory: %s", err), gc)
		return
	}
	build := repo.Builds[commit]
	for fname, file := range files {
		buildFolder := filepath.Join(STORAGE, commitDirectory, fname)
		log.Printf("%s/%s (%s): Saving to %s\n", ns, name, commit, buildFolder)
		if err := gc.SaveUploadedFile(file[0], buildFolder); err != nil {
			end(500, fmt.Sprintf("Couldn't store file: %s", err), gc)
			return
		}
	}
	build.DateChanged = time.Now()
	build.Files = commitDirectory
	repo.Builds[commit] = build
	app.storage[ns+"/"+name] = repo
	app.store()
	gc.AbortWithStatus(200)
}

func roundPageCount(c uint) uint {
	d := float64(c) / float64(BUILDSPERPAGE)
	return uint(math.Ceil(d))
}

func (app *appContext) getRepos(gc *gin.Context) {
	resp := map[string]RepoDTO{}
	for nsName, repo := range app.storage {
		nRepo := RepoDTO{
			Namespace: repo.Namespace,
			Name:      repo.Name,
			Secret:    (repo.Secret != ""),
		}
		newestCommit := ""
		newestTime := time.Time{}
		for commit, build := range repo.Builds {
			if build.Date.After(newestTime) {
				newestCommit = commit
				newestTime = build.Date
			}
		}
		nRepo.LatestCommit = newestCommit
		build := repo.Builds[newestCommit]
		nRepo.LatestPush = BuildDTO{
			ID:   build.ID,
			Link: build.Link,
			Date: build.Date,
		}
		resp[nsName] = nRepo
	}
	gc.JSON(200, resp)
}

type SortableBuilds struct {
	keys   []string
	builds map[string]BuildDTO
}

func (sb SortableBuilds) Len() int      { return len(sb.builds) }
func (sb SortableBuilds) Swap(i, j int) { sb.keys[i], sb.keys[j] = sb.keys[j], sb.keys[i] }
func (sb SortableBuilds) Less(i, j int) bool {
	a := sb.builds[sb.keys[i]]
	b := sb.builds[sb.keys[j]]
	return a.Date.After(b.Date)
}

type BuildsDTO struct {
	Order  []string
	Builds map[string]BuildDTO
}

func (app *appContext) getBuild(gc *gin.Context) {
	namespace := gc.Param("namespace")
	name := gc.Param("name")
	commit := gc.Param("build")
	repo, ok := app.storage[namespace+"/"+name]
	if !ok {
		end(400, fmt.Sprintf("Repository not found: %s/%s", namespace, name), gc)
		return
	}
	build, ok := repo.Builds[commit]
	if !ok {
		end(400, "Build not found", gc)
		return
	}
	gc.JSON(200, BuildDTO{
		ID:     build.ID,
		Name:   build.Name,
		Link:   build.Link,
		Date:   build.Date,
		Branch: build.Branch,
	})
}

func (app *appContext) getBuilds(gc *gin.Context) {
	namespace := gc.Param("namespace")
	name := gc.Param("name")
	page, err := strconv.Atoi(gc.Param("page"))
	if err != nil {
		end(400, fmt.Sprintf("%s/%s: Invalid page: %s", namespace, name, err), gc)
		return
	}
	repo, ok := app.storage[namespace+"/"+name]
	if !ok {
		end(400, fmt.Sprintf("Repository not found: %s/%s", namespace, name), gc)
		return
	}
	numOfPages := roundPageCount(uint(len(repo.Builds)))
	if uint(page) > numOfPages {
		end(400, fmt.Sprintf("%s/%s: Invalid page index", namespace, name), gc)
		return
	}
	sb := SortableBuilds{}
	sb.keys = make([]string, len(repo.Builds))
	sb.builds = map[string]BuildDTO{}
	i := 0
	for c, b := range repo.Builds {
		dto := BuildDTO{
			ID:     b.ID,
			Name:   b.Name,
			Link:   b.Link,
			Date:   b.Date,
			Branch: b.Branch,
		}
		if b.Files != "" {
			files, err := ioutil.ReadDir(filepath.Join(STORAGE, b.Files))
			if err != nil {
				log.Printf("%s/%s: Error reading \"%s\": %s\n", namespace, name, b.Files, err)
				continue
			}
			dto.Files = make([]FileDTO, len(files))
			for i, f := range files {
				dto.Files[i] = FileDTO{
					Name: f.Name(),
					Size: fileSize(f.Size()),
				}
			}
		}
		if c != "" {
			sb.keys[i] = c
			sb.builds[c] = dto
		}
		i++
	}
	sort.Sort(sb)
	var commits []string
	if uint(page) != numOfPages {
		commits = sb.keys[(page-1)*BUILDSPERPAGE : page*BUILDSPERPAGE]
	} else {
		commits = sb.keys[(page-1)*BUILDSPERPAGE:]
	}
	resp := BuildsDTO{}
	resp.Order = commits
	resp.Builds = map[string]BuildDTO{}
	for _, c := range commits {
		resp.Builds[c] = sb.builds[c]
	}
	gc.JSON(200, resp)
}

func (app *appContext) getRepo(gc *gin.Context) {
	namespace := gc.Param("namespace")
	name := gc.Param("name")
	repo, ok := app.storage[namespace+"/"+name]
	if !ok {
		end(400, fmt.Sprintf("Repository not found: %s/%s", namespace, name), gc)
		return
	}
	resp := RepoDTO{
		Namespace:      namespace,
		Name:           name,
		BuildPageCount: roundPageCount(uint(len(repo.Builds))),
		Branches:       repo.Branches,
	}
	gc.JSON(200, resp)
}

func (app *appContext) LatestCommit(gc *gin.Context) {
	namespace := gc.Param("namespace")
	name := gc.Param("name")
	repo, ok := app.storage[namespace+"/"+name]
	if !ok {
		end(400, fmt.Sprintf("Repository not found: %s/%s", namespace, name), gc)
		return
	}
	build, ok := repo.Builds[repo.LatestNonEmptyBuild]
	if !ok {
		end(500, "Couldn't find latest build", gc)
		return
	}
	gc.JSON(200, BuildDTO{
		ID:     build.ID,
		Name:   build.Name,
		Link:   build.Link,
		Date:   build.Date,
		Branch: build.Branch,
	})
}

func (app *appContext) findLatest(gc *gin.Context) {
	namespace := gc.Param("namespace")
	name := gc.Param("name")
	search := strings.ToLower(gc.Param("search"))
	if search == "" {
		end(400, "No file name/query provided", gc)
		return
	}
	repo, ok := app.storage[namespace+"/"+name]
	if !ok {
		end(400, fmt.Sprintf("Repository not found: %s/%s", namespace, name), gc)
		return
	}
	build, ok := repo.Builds[repo.LatestNonEmptyBuild]
	if !ok {
		end(500, "Couldn't find latest build", gc)
		return
	}
	files, err := os.ReadDir(filepath.Join(STORAGE, build.Files))
	if err != nil {
		end(500, "Couldn't read directory", gc)
		return
	}
	for _, file := range files {
		if strings.Contains(strings.ToLower(file.Name()), search) {
			gc.FileAttachment(filepath.Join(STORAGE, build.Files, file.Name()), file.Name())
			return
		}
	}
	end(500, "No matching file found", gc)
}

func (app *appContext) getFile(gc *gin.Context) {
	namespace := gc.Param("namespace")
	name := gc.Param("name")
	buildname := gc.Param("build")
	fname := gc.Param("file")
	repo, ok := app.storage[namespace+"/"+name]
	if !ok {
		end(400, fmt.Sprintf("Repository not found: %s/%s", namespace, name), gc)
		return
	}
	build, ok := repo.Builds[buildname]
	if !ok {
		end(400, "Build not found", gc)
		return
	}
	path := filepath.Join(build.Files, fname)
	if _, err := os.Stat(filepath.Join(STORAGE, path)); os.IsNotExist(err) {
		end(400, fmt.Sprintf("File not found: %s", path), gc)
		return
	}
	gc.FileFromFS(path, app.fs)
}
