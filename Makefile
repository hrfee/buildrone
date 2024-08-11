.PHONY: clean

all: build/buildrone

DEBUG ?= off
ifeq ($(DEBUG), on)
	TSARGS := --sourcemap
	TSCOPY := cp -r ts build/static/
else
	TSARGS := --minify
	TSCOPY :=
endif

build/static/upload.py: static/upload.py
	mkdir -p build
	cp -r static build/
	$(TSCOPY)
	cp -r templates build/

static/admin.js: $(wildcard ts/*) build/static/upload.py
	$(info compiling typescript)
	esbuild --target=es6 --bundle ts/admin.ts $(TSARGS) --outfile=build/static/admin.js 
	esbuild --target=es6 --bundle ts/repo.ts $(TSARGS) --outfile=build/static/repo.js 

build/buildrone: $(wildcard *.go) static/admin.js
	$(info grabbing deps)
	go mod download
	$(info compiling)
	mkdir -p build
	CGO_ENABLED=0 go build -o build/buildrone *.go

clean:
	rm -rf build
