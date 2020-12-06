console.log("Hello from status.js");
window.addEventListener('load', () => {
    let instance = window.location.href;
    fetch(`${instance}/status`)
        .then(res => {
            console.log({res});
            return res.json();
        })
        .then(data => console.log({data}))
});
