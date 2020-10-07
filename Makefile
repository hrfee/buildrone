typescript:
	esbuild ts/* --outdir=static --sourcemap
	cp -r ts static/
