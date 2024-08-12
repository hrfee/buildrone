#### *buildrone*

A small app for serving build output files publicly for Woodpecker CI, See the main branch for Drone CI support. You use it like this:
* Once your repo is setup in drone, open the buildrone dashboard and press "Setup" on your repo. A key is generated, which you store as the `BUILDRONE_SECRET` environment variable in your Drone build settings.
* In your CI config, grab the upload script from `your_buildrone_url/upload.py`, and run it to upload your files. 
* Working example of public ui and `upload.py` usage can be found [here](https://dl.jfa-go.com) and [here](https://github.com/hrfee/jfa-go/tree/main/.woodpecker) respectively.

#### *building/installing*
builds are of course provided by a [buildrone instance](https://builds.hrfee.pw/view/hrfee/buildrone), just extract and run. Building yourself is trivial also.

Run `npm i` to get node deps, and `make [DEBUG=on/off]` to compile css/ts and the program, outputting everything into the `build/` directory. `make clean` empties it.

A Dockerfile is also provided.
```
(main) >: docker build -t buildrone .

(main) >: docker create --name buildrone \
                        --restart always \
                        -v path/to/your/config.ini:/config.ini \
                        -v path/to/data/storage:/data \
                        -p 8062:8062 \
                        buildrone
```

#### *usage*
On first run, a template config file will be created. Fill it out then rerun the program again to start it. Daemonization is up to you.

```
Usage of buildrone:
  -config string
    	location of config file (ini) (default "~/.config/buildrone/config.ini")
  -data string
    	location of stored database and build files (default "~/.local/share/buildrone")
  -debug
    	use debug mode
  -host string
    	address to host app on (default "0.0.0.0")
  -maxage string
    	Delete files from commits once they are this old. 
        example: 1y30d2h (m = minutes, h = hours, d = days, y = years).
  -port int
    	port to host app on (default 8062)
```
