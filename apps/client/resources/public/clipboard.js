const sosCopyButton = document.querySelector("button#copy-sos-ssh")
      ? document.querySelector('button#copy-sos-ssh')
      : null;
const sosVal = document.querySelector('pre#sos-ssh')
      ? document.querySelector('pre#sos-ssh').textContent
      : null;

const tmateCopyButton = document.querySelector('button#copy-tmate-ssh')
      ? document.querySelector('button#copy-tmate-ssh')
      : null;
const tmateVal = document.querySelector('pre#tmate-ssh')
      ? document.querySelector('pre#tmate-ssh').textContent
      : null;

const kcCommandCopyButton = document.querySelector('button#copy-kc-command')
      ? document.querySelector('button#copy-kc-command')
      : null;
const kcCommandVal = document.querySelector('pre#kc-command')
      ? document.querySelector('pre#kc-command').textContent
      : null;

const kcCopyButton = document.querySelector('button#copy-kc')
      ? document.querySelector('button#copy-kc')
      : null;
const kcVal = document.querySelector('pre#kc')
      ? document.querySelector('pre#kc').textContent
      : null;


function copyToClipboard (str) {
    const el = document.createElement('textarea');
    el.value = str;
    document.body.appendChild(el);
    el.select();
    document.execCommand('copy');
    document.body.removeChild(el);
}

window.addEventListener('load', () => {
    if (sosCopyButton)  {
        sosCopyButton.addEventListener('click', () => copyToClipboard(sosVal));
    }
    if (tmateCopyButton) {
        tmateCopyButton.addEventListener('click', () => copyToClipboard(tmateVal));
    }
    if (kcCommandCopyButton) {
        kcCommandCopyButton.addEventListener('click', () => copyToClipboard(kcCommandVal));
    }
    if (kcCopyButton) {
        kcCopyButton.addEventListener('click', () => copyToClipboard(kcVal));
    }
});
