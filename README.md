#### *buildrone*

A small app for serving build output files publicly for Drone CI. You use it like this:
* Once your repo is setup in drone, open the buildrone dashboard and press "Setup" on your repo. A key is generated, which you store as the `BUILDRONE_SECRET` environment variable in your Drone build settings.
* In your drone.yml, grab the upload script from `your_buildrone_url/upload.py`, and run it to upload your files. 
* Working example of public ui and `upload.py` usage can be found [here](https://builds.hrfee.pw/view/hrfee/jfa-go) and [here](https://github.com/hrfee/jfa-go/blob/main/.drone.yml) respectively.

#### *building/installing*
builds are of course provided by a [buildrone instance](https://builds.hrfee.pw/view/hrfee/buildrone), just extract and run. Building yourself is trivial also.

Install esbuild and ensure its in your path, and then run `make all` to get deps, compile the program and typescript, and dump everything in the `build/` folder.

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
