interface Window {
    token: string;
}

function toClipboard(str: string): void {
    const el = document.createElement('textarea') as HTMLTextAreaElement;
    el.value = str;
    el.readOnly = true;
    el.style.position = "absolute";
    el.style.left = "-9999px";
    document.body.appendChild(el);
    const selected = document.getSelection().rangeCount > 0 ? document.getSelection().getRangeAt(0) : false;
    el.select();
    document.execCommand("copy");
    document.body.removeChild(el);
    if (selected) {
        document.getSelection().removeAllRanges();
        document.getSelection().addRange(selected);
    }
}

const _get = (url: string, data: Object, onreadystatechange: () => void): void => {
    let req = new XMLHttpRequest();
    req.open("GET", url, true);
    req.responseType = 'json';
    req.setRequestHeader("Authorization", "Bearer " + btoa(window.token));
    req.setRequestHeader('Content-Type', 'application/json');
    req.onreadystatechange = onreadystatechange;
    req.send(JSON.stringify(data));
};

const _post = (url: string, data: Object, onreadystatechange: () => void): void => {
    let req = new XMLHttpRequest();
    req.open("POST", url, true);
    req.responseType = 'json';
    req.setRequestHeader("Authorization", "Bearer " + btoa(window.token));
    req.setRequestHeader('Content-Type', 'application/json; charset=UTF-8');
    req.onreadystatechange = onreadystatechange;
    req.send(JSON.stringify(data));
};

function _delete(url: string, data: Object, onreadystatechange: () => void): void {
    let req = new XMLHttpRequest();
    req.open("DELETE", url, true);
    req.setRequestHeader("Authorization", "Bearer " + btoa(window.token));
    req.setRequestHeader('Content-Type', 'application/json; charset=UTF-8');
    req.onreadystatechange = onreadystatechange;
    req.send(JSON.stringify(data));
}

const rmAttr = (el: HTMLElement, attr: string): void => {
    if (el.classList.contains(attr)) {
        el.classList.remove(attr);
    }
};
const addAttr = (el: HTMLElement, attr: string): void => el.classList.add(attr);

const Focus = (el: HTMLElement): void => rmAttr(el, 'unfocused');
const Unfocus = (el: HTMLElement): void => addAttr(el, 'unfocused');

interface Repo {
    Namespace: string;
    Name: string;
    Builds: { [commit: string]: Build };
    LatestCommit: string;
    LatestPush: Build;
    Secret: boolean;
}

interface NewSecret {
    Secret: string;
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

const genCard = (repo: Repo): HTMLDivElement => {
    const hasBuilds = Object.keys(repo.Builds).length != 0
    let shortCommit = '';
    if (repo.Secret && hasBuilds && repo.LatestCommit.length >= 7) {
        shortCommit = repo.LatestCommit.substring(0, 7);
    }
    let link = `${base}/view/${repo.Namespace}/${repo.Name}`;  
    let repoSection = '';
    if (repo.Secret) {
        repoSection = `
        <a href="${link}" class="card-title h5">${repo.Namespace}/${repo.Name}</a>
        `;
        if (hasBuilds) { 
            repoSection += `<a href="${repo.LatestPush.Link}" class="card-title h5 text-monospace text-gray">${shortCommit}</a>
            <div class="card-subtitle text-gray">Last commit: ${repo.LatestPush.Date.toLocaleDateString('en-US')} @ ${repo.LatestPush.Date.toLocaleTimeString('en-US')}</div>
            `;
        } else {
            repoSection += `<div class="card-subtitle text-gray">No commits yet.</div>`;
        }
        repoSection += `
        `;
    } else {
        repoSection = `
        <a class="card-title h5 text-gray">${repo.Namespace}/${repo.Name}</a>
        <div class="card-subtitle text-gray">Not configured.</div>
        `;
    }
    let newSecretButton = "";
    if (repo.Secret) {
        newSecretButton = `
        <button class="btn btn-lg btn-error" onclick="newSecretWarning('${repo.Namespace}', '${repo.Name}', this)" style="margin: 0.5rem;">New Secret</button>
        `;
    }
    let text = `
    <div class="card container">
        <div class="columns col-gapless">
            <div class="column">
                <div class="card-header">
                    ${repoSection}
                </div>
            </div>
            <div class="divider-vert"></div>
            <div class="column">
                <div class="card-body" style="padding-bottom: 0.8rem;">
                    <button class="btn btn-lg ${!repo.Secret ? '' : 'btn-primary'}" onclick="newKey('${repo.Namespace}', '${repo.Name}', false, this)" style="margin: 0.5rem;">${!repo.Secret ? 'Setup' : 'New Key'}</button>
                    ${newSecretButton}
                    <div class="textArea"></div>
                </div>
            <div>
        </div>
    </div>
    `;
    const el = document.createElement('div') as HTMLDivElement;
    el.innerHTML = text;
    return el.firstElementChild as HTMLDivElement;
};

const emptyCard = (): HTMLDivElement => {
    const el = document.createElement('div') as HTMLDivElement;
    el.innerHTML = `
    <div class="card empty">
        <div class="empty-subtitle">
            Setup repositories in Drone to see them here.
        </div>
    </div>
    `;
    return el.firstElementChild as HTMLDivElement;
};


interface NewKeyReqDTO {
    NewSecret: boolean;
}

interface NewKeyRespDTO {
    Key: string;
}

var newSecretButtonEl: HTMLButtonElement;

function newSecretWarning(namespace: string, name: string, button: HTMLButtonElement): void {
    newSecretButtonEl = button;
    (document.getElementById('newSecretSubmit') as HTMLButtonElement).setAttribute('onclick', `newKey('${namespace}', '${name}', true, newSecretButtonEl)`);
    addAttr(document.getElementById('secretWarningModal'), "active");
}

function newKey(namespace: string, name: string, newSecret: boolean, button: HTMLButtonElement): void {
    const textArea = button.parentElement.getElementsByClassName('textArea')[0];
    textArea.textContent = '';
    const removeError = !button.classList.contains("btn-error");
    addAttr(button, "loading");
    const ogText = button.textContent;
    let data: NewKeyReqDTO = { NewSecret: newSecret };
    _post(`/repo/${namespace}/${name}/key`, data, function (): void {
        if (this.readyState == 4) {
            rmAttr(document.getElementById('secretWarningModal'), "active");
            rmAttr(button, "loading");
            if (this.status != 200) {
                addAttr(button, "btn-error");
                rmAttr(button, "btn-primary");
                button.textContent = "Failed";
                setTimeout((): void => {
                    addAttr(button, "btn-primary");
                    if (removeError) {
                        rmAttr(button, "btn-error");
                    }
                    button.textContent = ogText;
                }, 3000);
            } else {
                addAttr(button, "btn-success");
                rmAttr(button, "btn-primary");
                const secret = (<NewKeyRespDTO>this.response).Key;
                button.textContent = "Success";
                const secretButton = document.createElement('button');
                secretButton.classList.add("btn", "text-monospace");
                secretButton.setAttribute('style', 'width: 6rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; margin: 0.5rem;');
                const msg = document.createElement('p');
                addAttr(msg, "text-gray");
                if (newSecret) {
                    msg.textContent += `
                    A new secret has been generated. All previous build keys are now invalid.
                    `;
                }
                msg.textContent += `

                Click the above build key to copy it, and store it as the 'BUILDRONE_KEY' environment variable in Drone for the upload script to use.`;
                secretButton.innerHTML = `
                <i class="icon icon-copy"></i> ${secret}
                `;
                secretButton.onclick = (): void => {
                    toClipboard(secret);
                    const toast = document.createElement('div') as HTMLDivElement;
                    addAttr(toast, "toast");
                    const closeButton = document.createElement('button') as HTMLButtonElement;
                    closeButton.classList.add('btn', 'btn-clear', 'float-right');
                    closeButton.onclick = (): void => toast.remove();
                    toast.appendChild(closeButton);
                    toast.appendChild(document.createTextNode('Copied to clipboard.'));
                    msg.appendChild(toast);
                    setTimeout((): void => toast.remove(), 5000);
                };
                textArea.appendChild(secretButton);
                textArea.appendChild(msg);
                setTimeout((): void => {
                    secretButton.remove();
                    msg.remove();
                    rmAttr(button, "btn-primary");
                    rmAttr(button, "btn-success");
                    button.textContent = ogText;
                }, 60000);
            }
        }
    });
}

const base = window.location.origin;

let repoList: { [ns_name: string]: Repo } = {}; 
var repoOrder: Array<string> = [];

const loginModal = document.getElementById('loginModal') as HTMLDivElement;

function login(username: string, password: string, modal: boolean, run?: (arg0: number) => void): void {
    const req = new XMLHttpRequest();
    req.responseType = 'json';
    req.open("GET", "/token", true);
    req.setRequestHeader("Authorization", "Basic " + btoa(username + ":" + password));
    req.onreadystatechange = function (): void {
        if (this.readyState == 4) {
            const button = document.getElementById('loginButton') as HTMLButtonElement;
            rmAttr(button, "loading");
            if (this.status != 200) {
                let errorMsg = this.response["error"];
                if (!errorMsg) {
                    errorMsg = "Unknown error";
                }
                if (modal) {
                    button.disabled = false;
                    button.textContent = errorMsg;
                    addAttr(button, "btn-error");
                    rmAttr(button, "btn-primary");
                    setTimeout((): void => {
                        addAttr(button, "btn-primary");
                        rmAttr(button, "btn-error");
                        button.textContent = "Login";
                    }, 4000);
                } else {
                    addAttr(loginModal, "active");
                }
            } else {
                const data = this.response;
                window.token = data["token"];
                loadRepos();
                rmAttr(loginModal, "active");
            }
            if (run) {
                run(+this.status);
            }
        }
    };
    req.send();
}

(document.getElementById('loginForm') as HTMLFormElement).onsubmit = function (): boolean {
    const button = document.getElementById('loginButton') as HTMLButtonElement;
    addAttr(button, "loading");
    const username = (document.getElementById('username') as HTMLInputElement).value;
    const password = (document.getElementById('password') as HTMLInputElement).value;
    login(username, password, true, null);
    return false;
};

login("", "", false, (status: number): void => {
    if (!(status == 200 || status == 204)) {
        addAttr(loginModal, "active");
    }
});
    


const loadRepos = (): void => _get('/repos', null, function (): void {
    if (this.readyState == 4 && this.status == 200) {
        repoList = this.response;
        for (const key of Object.keys(repoList)) {
            let buildOrder: Array<string> = [];
            for (const bKey of Object.keys(repoList[key].Builds)) {
                repoList[key].Builds[bKey].Date = new Date(repoList[key].Builds[bKey].Date);
                buildOrder.push(bKey);
            }
            buildOrder = buildOrder.sort((a: string, b: string) => repoList[key].Builds[b].Date.getTime() - repoList[key].Builds[a].Date.getTime())
            repoList[key].LatestCommit = buildOrder[0];
            repoList[key].LatestPush = repoList[key].Builds[buildOrder[0]]
            repoOrder.push(key);
        }
        repoOrder = repoOrder.sort((a: string, b: string): any => {
            if (repoList[b].Secret == repoList[a].Secret) {
                if (repoList[b].Secret && Object.keys(repoList[b].Builds).length != 0 && Object.keys(repoList[a].Builds).length != 0) {
                    return repoList[b].LatestPush.Date.getTime() - repoList[a].LatestPush.Date.getTime();
                } else {
                    return 0;
                }
            } else {
                if (repoList[b].Secret) {
                    return 1;
                } else {
                    return -1;
                }
            }
        });
        const el = document.getElementById("content");
        for (let i = 0; i < repoOrder.length; i++) {
            el.appendChild(genCard(repoList[repoOrder[i]]));
        }
        if (repoOrder.length < 2) {
            el.appendChild(emptyCard())
        }
    }
});



