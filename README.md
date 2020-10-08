#### *buildrone*

A small app for serving build output files publicly for Drone CI. You use it like this:
* Once your repo is setup in drone, open the buildrone dashboard and press "Setup" on your repo. A key is generated, which you store as the `BUILDRONE_SECRET` environment variable in your Drone build srtings.
* In your drone.yml, grab the upload script from `your_buildrone_url/upload.py`, and run it to upload your files.




