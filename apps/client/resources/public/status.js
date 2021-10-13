const POLL_INTERVAL = 10000;

function isNotEmpty(value) {
  return value && typeof value !== "undefined" && value !== null;
}

function updateAge(instance) {
  let age = document.querySelector("em#age");
  if (isNotEmpty(instance.age)) {
    age.innerHTML = `Created by <a href="https://github.com/${instance.owner}">${instance.owner}</a> ${instance.age} ago`;
  }
}

function updateTmate(instance) {
  const tmateSection = document.querySelector("section#tmate");
  const ssh = document.querySelector("pre#tmate-ssh");
  const web = document.querySelector("a.tmate.action");
  if (isNotEmpty(instance["tmate-ssh"]) && isNotEmpty(instance["tmate-web"])) {
    tmateSection.classList.remove("hidden");
    ssh.textContent = instance["tmate-ssh"];
    web.href = instance["tmate-web"];
  }
}

function updateInstanceInfo(instance) {
  const phase = document.querySelector("h3#phase");
  const type = document.querySelector("p#type");
  const facility = document.querySelector("p#facility");
  const kubernetesNodeCount = document.querySelector("p#kubernetesNodeCount");
  if (isNotEmpty(instance.phase)) {
    phase.textContent = `Status: ${instance.phase}`;
  } else {
    phase.textContent = "Getting status...";
  }

  if (isNotEmpty(instance.type)) {
    type.textContent = `Type: ${instance.type}`;
  } else {
    type.textContent = "";
  }

  if (isNotEmpty(instance.facility)) {
    facility.textContent = `Facility: ${instance.facility}`;
  } else {
    facility.textContent = "";
  }

  if (isNotEmpty(instance.kubernetesNodeCount)) {
    kubernetesNodeCount.textContent = `Node count: ${
      instance.kubernetesNodeCount + 1
    }`;
  } else {
    kubernetesNodeCount.textContent = "";
  }
}

function updateSitesAvailable(instance) {
  const sitesList = document.querySelector("ul#sites-available");
  const sites = instance.sites;
  if (isNotEmpty(sites) && sites.count > 0) {
    while (sitesList.firstChild) {
      sitesList.removeChild(sitesList.firstChild);
    }
    sites.forEach((s) => {
      const site = document.createElement("li");
      site.innerHTML = `<a href="${s}">${s}</a>`;
      sitesList.appendChild(site);
    });
  }
}

function updateSOS(instance) {
  const sos = document.querySelector("pre#sos-ssh");
  if (isNotEmpty(instance.facility) && isNotEmpty(instance.uid)) {
    sos.textContent = `ssh ${instance.uid}@sos.${instance.facility}.platformequinix.com`;
  }
}

function updateKubeconfig(instance) {
  const dl = document.querySelector("a#kc-dl");
  const config = document.querySelector("pre#kc");
  if (isNotEmpty(instance.kubeconfig) && isNotEmpty(instance.uid)) {
    const publicLink = `https://${window.location.host}/public-instances/${instance.uid}/${instance["instance-id"]}/kubeconfig`;
    dl.href = publicLink;
    config.textContent = instance.kubeconfig;
  }
}

function updatePublicLink(instance) {
  const link = document.querySelector("a#public-link");
  if (isNotEmpty(instance.uid)) {
    const publicLink = `https://${window.location.host}/public-instances/${instance.uid}/${instance["instance-id"]}`;
    link.href = publicLink;
    link.classList.remove("hidden");
  }
}

function updateElements(instance) {
  updateAge(instance);
  updateTmate(instance);
  updateInstanceInfo(instance);
  updateSitesAvailable(instance);
  updateSOS(instance);
  updateKubeconfig(instance);
  updatePublicLink(instance);
}

function updateStatus() {
  if (!document.hidden) {
    // only do this if site is active tab
    const instance = window.location.href;
    fetch(`${instance}/status`)
      .then((res) => res.json())
      .then((data) => updateElements(data));
  }
}

window.addEventListener("load", () => {
  updateStatus();
  setInterval(updateStatus, POLL_INTERVAL);
});
