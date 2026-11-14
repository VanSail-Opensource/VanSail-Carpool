import type { CatmullRomCurve3, Group, Mesh, MeshStandardMaterial } from 'three'
import { demoThreeActive, demoThreeCapacityScale, demoThreeRouteState, demoThreeStory, type DemoThreeFrame, type DemoThreeRoute, type DemoThreeTopic } from './demoThreeModel'
import { createDemoThreeObject } from './demoThreeObjects'
import { demoThreePalette as colors, disposeDemoThreeGroup, type DemoThreeObject } from './demoThreePrimitives'

interface RouteObject { specification: DemoThreeRoute; curve: CatmullRomCurve3; material: MeshStandardMaterial; beads: Mesh[]; stop?: Group }
export interface DemoThreeRenderer { draw: (frame: DemoThreeFrame) => void; dispose: () => void }

/** Imported only after a tutorial opens. Rendering is driven by the shared player clock. */
export async function createDemoThreeRenderer(canvas: HTMLCanvasElement, initialTopic: DemoThreeTopic): Promise<DemoThreeRenderer> {
  const T = await import('three')
  if (!canvas.isConnected) throw new Error('tutorial_canvas_detached')
  // Probe first: unsupported WebGL must not invoke Three's console.error path.
  const context = canvas.getContext('webgl2', { alpha: true, antialias: true, powerPreference: 'low-power' })
  if (!context) throw new Error('webgl2_unavailable')
  const renderer = new T.WebGLRenderer({ canvas, context, alpha: true, antialias: true, powerPreference: 'low-power' })
  renderer.setPixelRatio(Math.min(globalThis.devicePixelRatio || 1, 1.5))
  renderer.setClearColor(0x000000, 0)
  const scene = new T.Scene(), content = new T.Group()
  scene.add(content, new T.HemisphereLight(0xd7f4ff, 0x243943, 2.8))
  const key = new T.DirectionalLight(0xffffff, 3.5); key.position.set(-3, 5, 6); scene.add(key)
  const fill = new T.DirectionalLight(0x8ccbd1, 2); fill.position.set(5, -2, 3); scene.add(fill)
  const camera = new T.OrthographicCamera(-4.9, 4.9, 2.7, -2.7, .1, 40)
  camera.position.set(0, 0, 12); camera.lookAt(0, 0, 0)
  let topic = initialTopic, disposed = false, width = 0, height = 0
  let objects = new Map<string, DemoThreeObject>(), routes: RouteObject[] = []

  function clear(): void {
    // Also release a symbol's unused materials, which never entered the scene graph.
    disposeDemoThreeGroup(content, [...objects.values()].flatMap(object => [...object.materials])); objects.clear(); routes = []
  }

  function build(nextTopic: DemoThreeTopic): void {
    clear(); topic = nextTopic
    const story = demoThreeStory(topic)
    objects = new Map(story.nodes.map(specification => {
      const object = createDemoThreeObject(T, specification.shape, specification.warning)
      object.group.position.set(specification.x, specification.y, 0)
      object.group.scale.setScalar(specification.scale ?? 1)
      content.add(object.group)
      return [specification.id, object]
    }))
    routes = story.routes.map(specification => {
      const from = story.nodes.find(node => node.id === specification.from)!, to = story.nodes.find(node => node.id === specification.to)!
      const curve = new T.CatmullRomCurve3([
        new T.Vector3(from.x, from.y, -.4),
        new T.Vector3((from.x + to.x) / 2, (from.y + to.y) / 2 + (specification.bend ?? 0), -.25),
        new T.Vector3(to.x, to.y, -.4),
      ])
      const material = new T.MeshStandardMaterial({ color: colors.track, emissive: colors.accent, emissiveIntensity: .08, roughness: .6 })
      content.add(new T.Mesh(new T.TubeGeometry(curve, 32, .022, 5, false), material))
      const beadMaterial = new T.MeshStandardMaterial({ color: specification.warning ? colors.warning : colors.accent, emissive: specification.warning ? colors.warning : colors.accent, emissiveIntensity: .7 })
      const beadGeometry = new T.SphereGeometry(.055, 10, 8)
      const beads = Array.from({ length: 3 }, () => { const bead = new T.Mesh(beadGeometry, beadMaterial); content.add(bead); return bead })
      let stop: Group | undefined
      if (specification.warning) {
        stop = new T.Group(); stop.position.copy(curve.getPoint(.76)); stop.position.z = .1
        for (const angle of [-Math.PI / 4, Math.PI / 4]) {
          const bar = new T.Mesh(new T.BoxGeometry(.045, .27, .045), beadMaterial); bar.rotation.z = angle; stop.add(bar)
        }
        content.add(stop)
      }
      return { specification, curve, material, beads, stop }
    })
  }

  function draw(frame: DemoThreeFrame): void {
    if (disposed || context!.isContextLost()) return
    if (frame.topic !== topic) build(frame.topic)
    const bounds = canvas.getBoundingClientRect()
    const nextWidth = Math.max(1, Math.round(bounds.width)), nextHeight = Math.max(1, Math.round(bounds.height))
    if (nextWidth !== width || nextHeight !== height) {
      width = nextWidth; height = nextHeight
      renderer.setSize(width, height, false)
      const halfWidth = Math.max(4.7, 2.7 * width / height)
      camera.left = -halfWidth; camera.right = halfWidth
      camera.top = halfWidth * height / width; camera.bottom = -camera.top
      camera.updateProjectionMatrix()
    }
    const phase = frame.reducedMotion ? .92 : Math.min(1, Math.max(0, frame.phase))
    const time = frame.step + phase, active = demoThreeActive(frame), story = demoThreeStory(topic)
    for (const specification of story.nodes) {
      const object = objects.get(specification.id)!, highlighted = active.includes(specification.id)
      const pulse = frame.reducedMotion ? 0 : Math.sin(time * Math.PI * 2 + specification.x) * .035
      const scale = (specification.scale ?? 1) * (highlighted ? 1.08 : .94) * demoThreeCapacityScale(frame, specification.id)
      object.group.scale.setScalar(scale)
      object.group.position.z = highlighted ? .12 + pulse : -.06
      object.group.rotation.y = -.25 + (highlighted && !frame.reducedMotion ? Math.sin(time * 1.3) * .09 : 0)
      object.body.color.setHex(highlighted ? colors.face : colors.body)
      object.accent.emissiveIntensity = highlighted ? .55 : .04
      object.accent.color.setHex(highlighted ? specification.warning ? colors.warning : colors.accent : 0x547681)
      if (object.hand) object.hand.rotation.z = -(time * Math.PI / 2)
    }
    routes.forEach((route, index) => {
      const { highlighted, flowing } = demoThreeRouteState(frame, route.specification)
      route.material.color.setHex(highlighted ? route.specification.warning ? colors.warning : 0x578d98 : colors.track)
      route.material.emissiveIntensity = highlighted ? .2 : 0
      route.beads.forEach((bead, beadIndex) => {
        const position = frame.reducedMotion ? (beadIndex + 1) / 4 : (phase * 1.2 + beadIndex / 3 + index * .08) % 1
        bead.visible = flowing && (!route.specification.warning || position < .7)
        bead.position.copy(route.curve.getPoint(position)); bead.position.z += .07
      })
      if (route.stop) route.stop.visible = highlighted
    })
    renderer.render(scene, camera)
  }

  function dispose(): void {
    if (disposed) return
    disposed = true; clear(); renderer.dispose(); renderer.forceContextLoss(); scene.clear()
  }
  try { build(initialTopic) } catch (error) { dispose(); throw error }
  return { draw, dispose }
}
