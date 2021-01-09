var locale: string = navigator.language || window.navigator.language || "en-US"

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
    Branches: Array<string>;
    Builds: { [commit: string]: BuildCard };
    BuildPageCount: number;
}

interface Build {
    ID: number;
    Name: string;
    Date: Date;
    Files: File[];
    Link: string;
    Branch: string;
}

interface File {
    Name: string;
    Size: string;
}

const base = window.location.href.split("/view")[0];
const title: Array<string> = document.title.split("/");
const namespace = title[0];
const repoName = title[1];

class BuildCard implements Build {
    private _card: HTMLDivElement;
    ID: number;
    Branch: string;
    private _files: File[];
    private _commit: string;
    private _commitLink: HTMLAnchorElement;
    private _buildPrefix: string;
    private _date: Date;

    get commit(): string { return this._commit; }
    set commit(c: string) { 
        this._commit = c;
        this._commitLink.textContent = c.substring(0, 7);
        this._buildPrefix = `${base}/repo/${repo.Namespace}/${repo.Name}/build/${c}`; 
    }

    get Link(): string { return this._commitLink.href; }
    set Link(l: string) { this._commitLink.href = l; }

    get Name(): string { return this._card.querySelector(".build-name").textContent; }
    set Name(n: string) { this._card.querySelector(".build-name").textContent = n; }

    get Date(): Date { return this._date; }
    set Date(d: Date) {
        this._date = d;
        const dateEl = this._card.querySelector(".build-date");
        dateEl.textContent = `${d.toLocaleDateString(locale)} @ ${d.toLocaleTimeString(locale)}`;
    }

    get Files(): File[] { return this._files; }
    set Files(f: File[]) {
        const dropdown = this._card.querySelector("input.build-dropdown") as HTMLInputElement;
        const accordion = this._card.querySelector(".accordion") as HTMLDivElement;
        const fileEl = this._card.querySelector(".build-files") as HTMLUListElement;
        if (f && f.length != 0) {
            this._files = f;
            let fileList = '';
            for (let file of f) {
                fileList += `
                <li class="menu-item">
                    <a href="${this._buildPrefix}/${file.Name}">${file.Name} <i class="menu-badge text-gray">${file.Size}</i></a>
                </li>
                `;
            }
            dropdown.checked = (f.length <= MAXFILESOPEN);
            fileEl.innerHTML = fileList;
            accordion.style.display = "";
        } else {
            accordion.parentElement.innerHTML = `<p class="text-gray">No files published for this commit.</p>`;
        }
    }

    constructor(build: Build, commit: string) {
        this._card = document.createElement("div") as HTMLDivElement;
        this._card.innerHTML = `
        <div class="card minicard">
            <div class="columns col-gapless">
                <div class="column">
                    <div class="card-header">
                        <a class="card-title h5 text-monospace build-commit"></a>
                        <div class="card-subtitle text-gray text-monospace build-name"></div>
                        <div class="card-subtitle text-gray build-date"></div>
                    </div>
                </div>
                <div class="divider-vert"></div>
                <div class="column">
                    <div class="card-body">
                        <div class="accordion">
                            <input type="checkbox" id="dropdown_${commit}" name="dropdown_${commit}" classlist="build-dropdown" hidden>
                            <label class="accordion-header" for="dropdown_${commit}">
                                <i class="icon icon-arrow-right mr-1"></i>
                                <a>Files</a>
                            </label>
                            <div class="accordion-body">
                                <ul class="menu menu-nav accordionList build-files"></ul>
                            </div>
                        </div>
                    </div>
                <div>
            </div>
        </div>
        `;
        this._commitLink = this._card.querySelector(".build-commit") as HTMLAnchorElement;
        this.commit = commit;
        this.ID = build.ID;
        this.Name = build.Name;
        this.Date = build.Date;
        this.Files = build.Files;
        this.Link = build.Link;
        this.Branch = build.Branch;
    }

    asElement = (): HTMLDivElement => { return this._card; }
}

interface BranchTab {
    Branch: string;
    tabEl: HTMLDivElement;
    buttonEl: HTMLAnchorElement;
}

const branchArea = document.getElementById("branch-area") as HTMLSpanElement;
const contentBox = document.getElementById('content') as HTMLDivElement;

class BranchTabs {
    private _current: string = "";
    tabs: Array<BranchTab>;
   
    constructor() {
        this.tabs = [];
    }

    tabEl = (branch: string): HTMLDivElement => {
        for (let t of this.tabs) {
            if (t.Branch == branch) {
                return t.tabEl;
            }
        }
    }

    addTab = (branch: string) => {
        let tab = {} as BranchTab;
        tab.Branch = branch;
        tab.tabEl = document.createElement("div") as HTMLDivElement;
        tab.tabEl.style.display = "none";
        contentBox.appendChild(tab.tabEl);
        tab.buttonEl = document.createElement("a") as HTMLAnchorElement;
        tab.buttonEl.classList.add("text-gray", "mr-1");
        tab.buttonEl.textContent = branch + " ";
        tab.buttonEl.onclick = () => { this.switch(branch); }
        branchArea.appendChild(tab.buttonEl);
        this.tabs.push(tab);
    }

    get current(): string { return this._current; }
    set current(Branch: string) { this.switch(Branch); }

    switch = (Branch: string) => {
        this._current = Branch;
        for (let t of this.tabs) {
            if (t.Branch == Branch) {
                t.tabEl.style.display = "";
                t.buttonEl.classList.remove("text-gray");
            } else {
                t.tabEl.style.display = "none";
                t.buttonEl.classList.add("text-gray");
            }
        }
    }
}


var repo: Repo;
var buildOrder: string[] = [];
var currentPage = 1;

_get(`${base}/repo/${namespace}/${repoName}`, null, function (): void {
    if (this.readyState == 4 && this.status == 200) {
        repo = this.response as Repo;
        repo.Builds = {};
        console.log(repo.Branches);
        for (let branch of repo.Branches) {
            if (branch == "main" || branch == "master") {
                branchTabs.addTab(branch);
            }
        }
        for (let branch of repo.Branches) {
            if (!(branch == "main" || branch == "master")) {
                branchTabs.addTab(branch);
            }
        }
        if (repo.BuildPageCount != 0) {
            getPage(1);
        }
    }
});

interface BuildsDTO {
    Order: string[];
    Builds: { [commit: string]: Build };
}

var currentBranch: string = "";
var branchTabs = new BranchTabs();

const getPage = (page: number): void => _get(`${base}/repo/${namespace}/${repoName}/builds/${page}`, null, function (): void {
    if (this.readyState == 4) {
        const loadButton = document.getElementById('loadMore') as HTMLButtonElement;
        if (this.status == 200) {
            currentPage = page;
            loadButton.remove();
            const resp: BuildsDTO = this.response;
            for (const key of resp.Order) {
                const build = resp.Builds[key];
                if ((build.Branch == "main" || build.Branch == "master") && currentBranch == "") {
                    currentBranch = build.Branch;
                    branchTabs.current = build.Branch;
                }
                let tabEl = branchTabs.tabEl(build.Branch);
                build.Date = new Date(build.Date as any);
                buildOrder.push(key);
                repo.Builds[key] = new BuildCard(build, key);
                tabEl.appendChild(repo.Builds[key].asElement());
            }
            if (currentBranch == "") {
                currentBranch = repo.Branches[0];
                branchTabs.current = repo.Branches[0];
            }
        }
        loadButton.onclick = (): void => getPage(currentPage+1);
        if (currentPage+1 <= repo.BuildPageCount) {
            contentBox.appendChild(loadButton);
        }
    }
});
    
