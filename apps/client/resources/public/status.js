console.log("Hello from status.js");

const POLL_INTERVAL = 7000;

// const statuses = ["age", "tmate-ssh", "tmate-web"]

// const statusElements = {
//     age: "em#age",
//     tmate: "section#tmate",
//     "tmate-ssh": "pre#tmate-ssh",
//     "tmate-web": "a.tmate"
// }

function updateAge (instance) {
    let age = document.querySelector("em#age");
    if (instance.age) {
        age.textContent = instance.age;
    }
}

function updateTmate (instance) {
    const tmateSection = document.querySelector('section#tmate');
    const ssh = document.querySelector('pre#tmate-ssh');
    const web = document.querySelector('a.tmate.action');
    if (instance["tmate-ssh"] && instance["tmate-web"]) {
        tmateSection.classList.remove('hidden');
        ssh.textContent = instance['tmate-ssh'];
        web.href = instance["tmate-web"];
    } else {
        console.log('no data for tmate!!!');
        console.log({instance});
    }
};

function updateInstanceInfo (instance) {
    let phase = document.querySelector('h3#phase');
    let type = document.querySelector('p#type');
    let facility = document.querySelector('p#facility');
    if (instance.phase) {
        phase.textContent = `Status: ${instance.phase}`;
    } else {
        phase.textContent = 'Getting status...'
    };

    if (instance.type) {
        type.textContent = `${instance.type} type`;
    } else {
        type.textContent  = '';
    };

    if (instance.facility) {
        facility.textContent = `deployed at ${instance.facility}`;
    } else {
        facility.textContent = '';
    };
};

function updateSitesAvailable (instance) {
    const sitesList = document.querySelector('ul#sites-available');
    let sites = instance.sites;
    if (sites.count > 0) {
        console.log({ sites });
        while (sitesList.firstChild) {
            sitesList.removeChild(sitesList.firstChild);
        }
        sites.forEach(s => {
            const site = document.createElement('li');
            site.innerHTML = `<a href="${s}">${s}</a>`
            sitesList.appendChild(site);
        });
    }
};

function updateElements (instance) {
    updateAge(instance);
    updateTmate(instance);
    updateInstanceInfo(instance);
    updateSitesAvailable(instance);
}

function updateStatus () {
    if (!document.hidden) {
        let instance = window.location.href;
        fetch(`${instance}/status`)
            .then(res => res.json())
            .then(data => updateElements(data));
    }
};

window.addEventListener('load', () => {
    setInterval(updateStatus, POLL_INTERVAL);
});
