const POLL_INTERVAL = 10000;

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
    }
};

function updateInstanceInfo (instance) {
    const phase = document.querySelector('h3#phase');
    const type = document.querySelector('p#type');
    const facility = document.querySelector('p#facility');
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
    const sites = instance.sites;
    if (sites.count > 0) {
        while (sitesList.firstChild) {
            sitesList.removeChild(sitesList.firstChild);
        }
        sites.forEach(s => {
            const site = document.createElement('li');
            site.innerHTML = `<a href="${s}">${s}</a>`;
            sitesList.appendChild(site);
        });
    }
};

function updateSOS (instance) {
    const sos = document.querySelector('pre#sos-ssh');
    if (instance.facility && instance.uid) {
        sos.textContent = `ssh ${instance.uid}@sos.${instance.facility}.platformequinix.com`;
    }
};

function updateKubeconfig (instance) {
    const dl = document.querySelector('a#kc-dl');
    const command = document.querySelector('pre#kc-command');
    const config = document.querySelector('pre#kc');
    if (instance.kubeconfig && instance.uid) {
        const publicLink = `https://${window.location.host}/public-instances/${instance.uid}/${instance["instance-id"]}/kubeconfig`
        dl.href = publicLink;
        command.textContent = `export KUBECONFIG=$(mktemp -t kubeconfig-XXXXX) ; curl -s ${publicLink} > "$KUBECONFIG"  ; kubectl api-resources`
        config.textContent = instance.kubeconfig;
    }
};

function updatePublicLink (instance) {
    const link = document.querySelector('a#public-link');
    if (instance.uid) {
        const publicLink = `https://${window.location.host}/public-instances/${instance.uid}/${instance["instance-id"]}`
        link.href=publicLink;
        link.classList.remove('hidden');
    }
}

function updateElements (instance) {
    updateAge(instance);
    updateTmate(instance);
    updateInstanceInfo(instance);
    updateSitesAvailable(instance);
    updateSOS(instance);
    updateKubeconfig(instance);
    updatePublicLink(instance);
}

function updateStatus () {
    if (!document.hidden) {// only do this if site is active tab
        const instance = window.location.href;
        fetch(`${instance}/status`)
            .then(res => res.json())
            .then(data => updateElements(data));
    }
};

window.addEventListener('load', () => {
    setInterval(updateStatus, POLL_INTERVAL);
});
