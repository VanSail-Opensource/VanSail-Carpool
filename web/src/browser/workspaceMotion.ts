/** Animate newly encountered section headings without hiding controls or data. */
export function installWorkspaceMotion(root: HTMLElement): () => void {
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)')
  const seen = new WeakSet<Element>()
  const animations = new Set<Animation>()
  const observer = new IntersectionObserver(entries => {
    for (const entry of entries) {
      if (!entry.isIntersecting) continue
      observer.unobserve(entry.target)
      if (reduced.matches || document.hidden || entry.boundingClientRect.bottom <= 0) continue
      const animation = entry.target.animate(
        [{ transform: 'translateY(8px)' }, { transform: 'none' }],
        { duration: 500, easing: 'cubic-bezier(.23, 1, .32, 1)' },
      )
      animations.add(animation)
      animation.onfinish = () => animations.delete(animation)
    }
  }, { threshold: 0, rootMargin: '0px 0px 96px 0px' })
  const scan = () => {
    root.querySelectorAll('.admin-section > h3, .admin-section > h4').forEach(element => {
      if (seen.has(element)) return
      seen.add(element)
      if (element.getBoundingClientRect().top >= window.innerHeight) observer.observe(element)
    })
  }
  const mutations = new MutationObserver(scan)
  const cancel = () => { animations.forEach(animation => animation.cancel()); animations.clear() }
  const onPreference = () => { if (reduced.matches) cancel() }
  reduced.addEventListener('change', onPreference)
  mutations.observe(root, { childList: true, subtree: true })
  scan()
  return () => { mutations.disconnect(); observer.disconnect(); cancel(); reduced.removeEventListener('change', onPreference) }
}
