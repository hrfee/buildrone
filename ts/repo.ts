const _get = (url: string, data: Object, onreadystatechange: () => void): void => {
    let req = new XMLHttpRequest();
    req.open("GET", url, true);
    req.responseType = 'json';
    //req.setRequestHeader("Authorization", "Basic " + btoa(window.token + ":"));
    req.setRequestHeader('Content-Type', 'application/json; charset=UTF-8');
    req.onreadystatechange = onreadystatechange;
    req.send(JSON.stringify(data));
};

const _post = (url: string, data: Object, onreadystatechange: () => void): void => {
    let req = new XMLHttpRequest();
    req.open("POST", url, true);
    //req.setRequestHeader("Authorization", "Basic " + btoa(window.token + ":"));
    req.setRequestHeader('Content-Type', 'application/json; charset=UTF-8');
    req.onreadystatechange = onreadystatechange;
    req.send(JSON.stringify(data));
};

function _delete(url: string, data: Object, onreadystatechange: () => void): void {
    let req = new XMLHttpRequest();
    req.open("DELETE", url, true);
    //req.setRequestHeader("Authorization", "Basic " + btoa(window.token + ":"));
    req.setRequestHeader('Content-Type', 'application/json; charset=UTF-8');
    req.onreadystatechange = onreadystatechange;
    req.send(JSON.stringify(data));
}

// Files accordion will be opened if number of files is equal to or below this.
const MAXFILESOPEN = 2;

interface Repo {
    Namespace: string;
    Name: string;
    Builds: { [commit: string]: Build };
}

interface Build {
    ID: number;
    Name: string;
    Date: Date;
    Files: Array<File>
    Link: string;
}

interface File {
    Name: string;
    Size: string;
}

const genCard = (repo: Repo, commit: string, latest: boolean): HTMLDivElement => {
    const build = repo.Builds[commit];
    const shortCommit = commit.substring(0, 7);
    let linkPrefix = `${base}/repo/${repo.Namespace}/${repo.Name}/build/${commit}`  
    let fileList: string = '';
    if (build.Files == null || build.Files.length == 0) {
        fileList = `
        <p class="text-gray">No files published for this commit.</p>
        `;
    } else {
        fileList = `
        <div class="accordion">
            <input type="checkbox" id="files_${commit}" name="files_${commit}" hidden ${(build.Files.length <= MAXFILESOPEN) ? 'checked' : ''}>
            <label class="accordion-header" for="files_${commit}">
                <i class="icon icon-arrow-right mr-1"></i>
                <a>Files</a>
            </label>
            <div class="accordion-body">
                <ul class="menu menu-nav">
        `;
        for (let i = 0; i < build.Files.length; i++) {
            fileList += `
            <li class="menu-item">
                <a href="${linkPrefix}/${build.Files[i].Name}">${build.Files[i].Name} <i class="menu-badge text-gray">${build.Files[i].Size}</i></a>
            </li>
            `;
        }
        fileList += `
                </ul>
            </div>
        </div>
        `;
    }
    let text = `
    <div class="card container">
        <div class="columns col-gapless">
            <div class="column">
                <div class="card-header">
                    <a href="${build.Link}" class="card-title h5 text-monospace">${shortCommit}</a>
                    <div class="card-subtitle text-gray text-monospace">${name}</div>
                    <div class="card-subtitle text-gray">${latest ? 'Latest ' : ''}(${build.Date.toLocaleDateString('en-US')} @ ${build.Date.toLocaleTimeString('en-US')})</div>
                </div>
            </div>
            <div class="divider-vert"></div>
            <div class="column">
                <div class="card-body">
                    ${fileList}
                </div>
            <div>
        </div>
    </div>
    `;
    const el = document.createElement('div') as HTMLDivElement;
    el.innerHTML = text;
    return el.firstElementChild as HTMLDivElement;
};

const base = window.location.href.split("/view")[0];
const title: Array<string> = document.title.split("/");
const namespace = title[0];
const repoName = title[1];

var repo: Repo;
var buildOrder: Array<string> = [];

_get(`${base}/repo/${namespace}/${repoName}`, null, function (): void {
    if (this.readyState == 4 && this.status == 200) {
        repo = <Repo>this.response;
        for (const key of Object.keys(repo.Builds)) {
            const build = repo.Builds[key];
            build.Date = new Date(build.Date);
            buildOrder.push(key);
        }
        buildOrder = buildOrder.sort((a: string, b: string) => repo.Builds[b].Date.getTime() - repo.Builds[a].Date.getTime())
        console.log(repo);
        for (let i = 0; i < buildOrder.length; i++) {
            document.getElementById("content").appendChild(genCard(repo, buildOrder[i], false));
        }
    }
});



