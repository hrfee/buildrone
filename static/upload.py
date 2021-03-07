import requests, os, sys, subprocess, argparse, base64, time
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument("url", help="url of buildrone instance")
parser.add_argument("namespace", help="namespace of repo (usually account's username)")
parser.add_argument("repo", help="name of repo")
parser.add_argument("--upload", help="files to upload", nargs="+")
parser.add_argument("--tag", help="<tagname>=<true>|<false>")

args = parser.parse_args()


try:
    KEY = os.environ["BUILDRONE_KEY"]
except KeyError:
    print("No API key provided. Run with BUILDRONE_KEY=<apikey>.")
    sys.exit(1)

tokenHeader = {"Authorization": f"Bearer {base64.b64encode(KEY.encode()).decode()}"}

tokenReq = requests.get(
    f"{args.url}/repo/{args.namespace}/{args.repo}/token", headers=tokenHeader
)
if tokenReq.status_code == 200:
    if tokenReq.json()["token"]:
        token = tokenReq.json()["token"]
        tokenHeader = {
            "Authorization": f"Bearer {base64.b64encode(token.encode()).decode()}"
        }
else:
    print(f"Token could not be fetched: {tokenReq}")
    sys.exit(1)


commit = subprocess.check_output("git rev-parse HEAD".split()).decode("utf-8").rstrip()

# args: 1 is url, 2 is namespace, 3 is repo name, rest are files/folders


def upload(filenames, namespace, repo, commit):
    handlers = []
    try:
        files = {}
        for name in filenames:
            if os.path.isfile(name):
                f = open(name, "rb")
                files[Path(name).name] = f
                print(f"Adding {name}")
                handlers.append(f)
        url = f"{args.url}/repo/{namespace}/{repo}/commit/{commit}/add"
        print(url)
        req = requests.post(url, headers=tokenHeader, files=files)
        print(f"Status {req}")
    finally:
        for h in handlers:
            h.close()


def tag(namespace, repo, commit, tagName, ready):
    try:
        version = subprocess.check_output(
            "git describe --exact-match HEAD".split()
        ).decode("utf-8")
    except subprocess.CalledProcessError:
        version = commit
    req = requests.post(
        f"{args.url}/repo/{namespace}/{repo}/commit/{commit}/tag/{tagName}",
        headers=tokenHeader,
        json={"ready": ready, "version": version, "date": str(int(time.time()))},
    )
    print(f"Status {req}")


if args.upload:
    for name in args.upload:
        if os.path.isdir(name):
            upload(
                [str(p) for p in Path(name).iterdir() if not os.path.isdir(p)],
                args.namespace,
                args.repo,
                commit,
            )
        else:
            upload([name], args.namespace, args.repo, commit)

if args.tag:
    (tagName, stateStr) = args.tag.split("=")
    state = False
    if stateStr == "true":
        state = True
    tag(args.namespace, args.repo, commit, tagName, state)
