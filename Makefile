.PHONY: clean

STATIC := build/static

all: $(STATIC)/admin.js build/buildrone

DEBUG ?= off
ifeq ($(DEBUG), on)
	TSARGS := --sourcemap
	TSCOPY := cp -r ts $(STATIC)/
	UNCSS :=
else
	TSARGS := --minify
	TSCOPY :=
	UNCSS := npx uncss $(wildcard templates/*.html) --stylesheets $(STATIC)/remixicon.css > $(STATIC)/_remixicon.css; mv $(STATIC)/_remixicon.css $(STATIC)/remixicon.css
endif

$(STATIC)/upload.py: static/upload.py $(wildcard templates/*.html)
	mkdir -p build
	cp -r static build/
	$(TSCOPY)
	cp -r templates build/

REMIXICON_SRC = node_modules/remixicon/fonts/remixicon.css node_modules/remixicon/fonts/remixicon.woff2
$(STATIC)/remixicon.css: $(REMIXICON_SRC) # $(wildcard *.html)
	cp $(REMIXICON_SRC) $(STATIC)/
	# $(UNCSS)

$(STATIC)/bundle.css: $(wildcard css/*) $(wildcard templates/*.html) $(wildcard ts/*)
	npx esbuild --bundle css/base.css --outfile=$(STATIC)/bundle.css --external:remixicon.css --minify
	npx tailwindcss -c tailwind.config.js -i $(STATIC)/bundle.css -o $(STATIC)/bundle.css

$(STATIC)/admin.js: $(wildcard ts/**) $(STATIC)/upload.py $(STATIC)/remixicon.css $(STATIC)/bundle.css
	$(info compiling typescript)
	npx esbuild --target=es6 --bundle ts/admin.ts $(TSARGS) --outfile=$(STATIC)/admin.js 
	npx esbuild --target=es6 --bundle ts/repo.ts $(TSARGS) --outfile=$(STATIC)/repo.js 

build/buildrone: $(wildcard *.go)
	$(info grabbing deps)
	go mod download
	$(info compiling)
	mkdir -p build
	CGO_ENABLED=0 go build -o build/buildrone *.go

clean:
	rm -rf build
