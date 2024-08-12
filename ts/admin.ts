import { Modal } from "./modules/modal.js";
import { addLoader, removeLoader, toClipboard, whichAnimationEvent } from "./modules/common.js";

window.animationEvent = whichAnimationEvent();
var locale: string = navigator.language || window.navigator.language || "en-US"

const _get = (url: string, data: Object, onreadystatechange: (req: XMLHttpRequest) => void): void => {
    let req = new XMLHttpRequest();
    req.open("GET", url, true);
    req.responseType = 'json';
    req.setRequestHeader("Authorization", "Bearer " + btoa(window.token));
    req.setRequestHeader('Content-Type', 'application/json');
    req.onreadystatechange = () => onreadystatechange(req);
    req.send(JSON.stringify(data));
};

const _post = (url: string, data: Object, onreadystatechange: (req: XMLHttpRequest) => void): void => {
    let req = new XMLHttpRequest();
    req.open("POST", url, true);
    req.responseType = 'json';
    req.setRequestHeader("Authorization", "Bearer " + btoa(window.token));
    req.setRequestHeader('Content-Type', 'application/json; charset=UTF-8');
    req.onreadystatechange = () => onreadystatechange(req);
    req.send(JSON.stringify(data));
};

function _delete(url: string, data: Object, onreadystatechange: (req: XMLHttpRequest) => void): void {
    let req = new XMLHttpRequest();
    req.open("DELETE", url, true);
    req.setRequestHeader("Authorization", "Bearer " + btoa(window.token));
    req.setRequestHeader('Content-Type', 'application/json; charset=UTF-8');
    req.onreadystatechange = () => onreadystatechange(req);
    req.send(JSON.stringify(data));
}

const rmAttr = (el: HTMLElement, attr: string): void => {
    if (el.classList.contains(attr)) {
        el.classList.remove(attr);
    }
};
const addAttr = (el: HTMLElement, attr: string): void => el.classList.add(attr);

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

class RepoCard {
    private _repo: Repo = {};
    private _card: HTMLElement;

    private _name: HTMLAnchorElement;
    private _commit: HTMLAnchorElement;
    private _latestPush: HTMLElement;

    private _newKey: HTMLButtonElement;
    private _newSecret: HTMLButtonElement;

    private _keyArea: HTMLElement;

    asElement = (): HTMLElement =>  { return this._card };

    constructor(repo: Repo) {
        this._card = document.createElement("div");
        this._card.classList.add("card", "flex", "flex-row", "flex-wrap", "gap-4", "justify-between");
        this._card.innerHTML = `
        <div class="flex flex-col gap-2 justify-between">
            <div class="heading flex flex-col gap-2 justify-between">
                <a class="repo-name hover:underline"></a> <a class="repo-commit font-mono text-neutral-400 hover:underline"></a>
            </div>
            <div class="repo-latest-push content"></div>
        </div>
        <div class="flex flex-row gap-4 justify-end">
            <div class="flex flex-col gap-2 justify-between max-w-sm min-w-xs repo-key-area card ~positive @low hidden"></div>
            <div class="flex flex-col justify-between gap-2">
                <button class="button ~urge @low repo-new-key grow"></button>
                <button class="button ~critical @low repo-new-secret grow hidden">New secret</button>
            </div>
        </div>
        `;
        this._name = this._card.getElementsByClassName("repo-name")[0] as HTMLAnchorElement;
        this._commit = this._card.getElementsByClassName("repo-commit")[0] as HTMLAnchorElement;
        this._latestPush = this._card.getElementsByClassName("repo-latest-push")[0] as HTMLAnchorElement;
       
        this._newKey = this._card.getElementsByClassName("repo-new-key")[0] as HTMLButtonElement;
        this._newKey.onclick = () => this.newKey(false);
        this._newSecret = this._card.getElementsByClassName("repo-new-secret")[0] as HTMLButtonElement;
        this._newSecret.onclick = this.newSecret;

        this._keyArea = this._card.getElementsByClassName("repo-key-area")[0] as HTMLElement;
        
        this.update(repo);
    }
    
    get namespace(): string { return this._repo.Namespace; };
    set namespace(v: string) {
        this._repo.Namespace = v;
        if (this.name != "") this.updateName();
    }

    get name(): string { return this._repo.Name; };
    set name(v: string) {
        this._repo.Name = v;
        if (this.namespace != "") this.updateName();
    }

    updateName = () => {
        this._name.textContent = `${this.namespace}/${this.name}`;
        this._name.href = `${window.location.origin}/view/${this._name.textContent}`;
        this._name.classList.remove("text-neutral-400");
    }

    get commit(): string { return this._repo.LatestCommit; };
    set commit(v: string) {
        if (v == "") {
            this._latestPush.textContent = `No commits yet.`;
            return;
        }
        this._commit.textContent = v.substring(0, 7);
    }

    set secret(v: boolean) {
        this._repo.Secret = v;
        if (!this._repo.Secret) {
            this.blankRepo();
        } else {
            this._newSecret.classList.remove("hidden");
            this._newKey.textContent = "New key";
            /* this._newKey.classList.add("@high");
            this._newKey.classList.remove("@low"); */
        }
    }

    blankRepo = () => {
        this._commit.textContent = "";
        this._commit.href = "";
        this._name.classList.add("text-neutral-400");
        this._latestPush.textContent = `Not configured.`;
        this._latestPush.classList.add("text-neutral-400");
        this._newSecret.classList.add("hidden");
        this._newKey.textContent = "Set up";
        /* this._newKey.classList.add("@low");
        this._newKey.classList.remove("@high"); */
    }

    get latestPush(): Build { return this._repo.LatestPush; };
    set latestPush(b: Build) {
        this._repo.LatestPush = b;
        this._commit.href = this._repo.LatestPush.Link;
        this._latestPush.textContent = `Last commit: ${this._repo.LatestPush.Date.toLocaleDateString(locale)} @ ${this._repo.LatestPush.Date.toLocaleTimeString(locale)}`;
    }

    newSecret = () => {
        (document.getElementById("secret-warning-submit") as HTMLButtonElement).onclick = () => this.newKey(true);
        secretWarningModal.show();
    };

    newKey = (secret: boolean) => {
        const button = secret ? this._newSecret : this._newKey;
        const ogText = button.textContent;
        this._keyArea.textContent = '';
        addLoader(button);
        let send: NewKeyReqDTO = { NewSecret: secret };
        _post(`/repo/${this.namespace}/${this.name}/key`, send, (req: XMLHttpRequest) => {
            if (req.readyState != 4) return;
            secretWarningModal.close();
            removeLoader(button);
            if (req.status != 200) {
                button.classList.add("~warning");
                button.textContent = `Failed`;
                setTimeout(() => {
                    button.classList.remove("~warning");
                    button.textContent = ogText;
                }, 5000);
                return;
            }
            let key = (req.response as NewKeyRespDTO).Key;
            this.secret = true;
            this._keyArea.innerHTML = `
            <p class="content">
                ${secret ? "Secret" : "New build key"} generated.
                ${secret ? "All previous build keys have been invalidated." : ""}
                ${!secret ? "Click below to copy, then set as the BUILDRONE_KEY environment variable for the upload script with a secret in the CI.": ""}
            </p>
            <div class="flex flex-row justify-center">
                <button class="button ~positive @low repo-copy-key flex flex-row gap-2 max-w-min">Copy<i class="ri-file-copy-line"></i></button>
            </div>
            `;
            this._keyArea.classList.remove("hidden");
            const copyButton = this._keyArea.getElementsByClassName("repo-copy-key")[0] as HTMLButtonElement;
            copyButton.onclick = () => {
                toClipboard(key);
                copyButton.classList.add("@high");
                copyButton.classList.remove("@low");
                copyButton.innerHTML = `Copied<i class="ri-check-line"></i>`;
                setTimeout(() => {
                    copyButton.classList.add("@low");
                    copyButton.classList.remove("@high");
                    copyButton.innerHTML = `Copy<i class="ri-file-copy-line"></i>`;
                }, 5000);
            }
        });
    }

    update = (r: Repo) => {
        this._repo = r;
        this.namespace = r.Namespace;
        this.name = r.Name;
        this.commit = r.LatestCommit;
        this.latestPush = r.LatestPush;
        this.secret = r.Secret;
    };
}

const emptyCard = (): HTMLDivElement => {
    const el = document.createElement('div') as HTMLDivElement;
    el.classList.add("card", "flex", "flex-row", "flex-wrap", "gap-4", "justify-center");
    el.innerHTML = `
    <div class="flex flex-col gap-2">
        <h5 class="heading">No repos</h5>
        <p class="content">Set up some repos in Drone/Woodpecker to see them here.</p>
    </div>
    `;
    return el;
};

interface NewKeyReqDTO {
    NewSecret: boolean;
}

interface NewKeyRespDTO {
    Key: string;
}

const secretWarningModal = new Modal(document.getElementById("secretWarningModal"));
(document.getElementById("secret-warning-close") as HTMLButtonElement).onclick = secretWarningModal.close;

let repoList: { [ns_name: string]: Repo } = {}; 
var repoOrder: Array<string> = [];

const loginModal = new Modal(document.getElementById("loginModal"), true);

function login(username: string, password: string, modal: boolean, run?: (arg0: number) => void): void {
    const req = new XMLHttpRequest();
    req.responseType = 'json';
    req.open("GET", "/token", true);
    req.setRequestHeader("Authorization", "Basic " + btoa(username + ":" + password));
    req.onreadystatechange = function (): void {
        if (this.readyState == 4) {
            const button = document.getElementById('loginButton') as HTMLButtonElement;
            removeLoader(button);
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
                    loginModal.show();
                }
            } else {
                const data = this.response;
                window.token = data["token"];
                loadRepos();
                loginModal.close();
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
    addLoader(button);
    const username = (document.getElementById('username') as HTMLInputElement).value;
    const password = (document.getElementById('password') as HTMLInputElement).value;
    login(username, password, true, null);
    return false;
};

login("", "", false, (status: number): void => {
    if (!(status == 200 || status == 204)) {
        loginModal.show();
    }
});

const loadRepos = (): void => _get('/repos', null, (req: XMLHttpRequest) => {
    if (req.readyState == 4 && req.status == 200) {
        repoList = req.response;
        for (const key of Object.keys(repoList)) {
            repoList[key].LatestPush.Date = new Date(repoList[key].LatestPush.Date as any);
            repoOrder.push(key);
        }
        repoOrder = repoOrder.sort((a: string, b: string): any => {
            if (repoList[b].Secret == repoList[a].Secret) {
                if (repoList[b].Secret && repoList[b].LatestCommit != "" && repoList[a].LatestCommit != "") {
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
        const el = document.getElementById("repos");
        for (let i = 0; i < repoOrder.length; i++) {
            el.appendChild((new RepoCard(repoList[repoOrder[i]])).asElement());
        }
        // No clue why 2, but we'll leave it i guess
        if (repoOrder.length < 2) {
            el.appendChild(emptyCard())
        }
    }
});
