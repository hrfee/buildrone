typescript:
	$(info compiling/minifying typescript)
	esbuild ts/* --outdir=static --minify

ts-debug:
	$(info compiling typescript with sourcemaps)
	esbuild ts/* --outdir=static --sourcemap
	cp -r ts static/


compile:
	$(info grabbing deps)
	go mod download
	$(info compiling)
	mkdir -p build
	CGO_ENABLED=0 go build -o build/buildrone *.go

copy:
	cp -r templates static build/

all: typescript compile copy
debug: ts-debug compile copy
