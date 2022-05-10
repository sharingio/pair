function GetObjectByQuerySelector(selector) {
  return (document.querySelector(selector)
          ? document.querySelector(selector)
          : null)
}

function GetObjectValueByQuerySelector(selector) {
  return (document.querySelector(selector)
          ? document.querySelector(selector).textContent
          : null)
}

const pasteButtons = [
  {
    button: "button#copy-sos-ssh",
    value: "pre#sos-ssh",
  },
  {
    button: "button#copy-tmate-ssh",
    value: "pre#tmate-ssh"
  },
  {
    button: "button#copy-kc-command",
    value: "pre#kc-command"
  },
  {
    button: "button#copy-kc",
    value: "pre#kc"
  },
  {
    button: "button#copy-pair-ssh-instance-command",
    value: "pre#pair-ssh-instance-command"
  }
]

function copyToClipboard (str) {
  const el = document.createElement('textarea');
  el.value = str;
  document.body.appendChild(el);
  el.select();
  document.execCommand('copy');
  document.body.removeChild(el);
}

window.addEventListener('load', () => {
  pasteButtons.map(b => {
    GetObjectByQuerySelector(b.button).addEventListener('click', () => copyToClipboard(GetObjectValueByQuerySelector(b.value)))
  })
})
