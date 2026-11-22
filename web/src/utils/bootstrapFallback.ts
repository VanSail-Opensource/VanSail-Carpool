import en from '../../locales/en/recovery.json'
import zhCN from '../../locales/zh-CN/recovery.json'

function recoveryCopy() {
  const languages = typeof navigator === 'undefined' ? [] : navigator.languages
  return languages.some((language) => language.toLowerCase().startsWith('zh')) ? zhCN.recovery : en.recovery
}

export function showBootstrapFailure(): void {
  const root = document.querySelector<HTMLElement>('#app')
  if (!root) return
  const copy = recoveryCopy()
  const main = document.createElement('main')
  const heading = document.createElement('h1')
  const description = document.createElement('p')
  const reload = document.createElement('button')
  main.className = 'auth-screen'
  main.setAttribute('role', 'alert')
  heading.textContent = copy.title
  description.textContent = copy.description
  reload.type = 'button'
  reload.textContent = copy.reload
  reload.addEventListener('click', () => window.location.reload())
  main.append(heading, description, reload)
  while (root.firstChild) root.removeChild(root.firstChild)
  root.appendChild(main)
}
