import type { DemoThreeShape } from './demoThreeModel'
import { demoThreePalette as colors, demoThreePrimitives, type DemoThreeObject, type ThreeLibrary } from './demoThreePrimitives'

export function createDemoThreeObject(T: ThreeLibrary, shape: DemoThreeShape, warning = false): DemoThreeObject {
  const group = new T.Group()
  const body = new T.MeshStandardMaterial({ color: colors.body, metalness: .48, roughness: .36 })
  const accent = new T.MeshStandardMaterial({ color: warning ? colors.warning : colors.accent, emissive: warning ? colors.warning : colors.accent, emissiveIntensity: .2, metalness: .35, roughness: .28 })
  const face = new T.MeshStandardMaterial({ color: colors.face, metalness: .4, roughness: .34 })
  const p = demoThreePrimitives(T, group, body)
  let hand: DemoThreeObject['hand']

  switch (shape) {
    case 'phone': {
      p.plate(.8, 1.35, .17)
      p.plate(.65, 1.05, .025, face, .05).position.z = .11
      p.box(.22, .025, .025, 0, -.58, .13, accent)
      p.box(.42, .055, .035, 0, .28, .14, accent)
      p.box(.42, .035, .035, 0, .09, .14, accent)
      p.box(.26, .035, .035, -.08, -.05, .14, accent)
      p.sphere(.085, .15, -.28, .15, accent)
      break
    }
    case 'server':
      for (const y of [-.37, 0, .37]) {
        p.plate(1.05, .27, .5).position.y = y
        p.sphere(.045, -.34, y, .28, accent)
        p.box(.35, .035, .025, .17, y, .28, accent)
      }
      break
    case 'globe': {
      p.sphere(.55, 0, 0, 0, face)
      p.ring(.56, .021, accent)
      p.ring(.56, .021, accent).rotation.y = Math.PI / 2
      p.ring(.56, .021, accent).rotation.x = Math.PI / 2
      p.ring(.56, .018, accent).rotation.y = Math.PI / 4
      break
    }
    case 'home': {
      p.box(.86, .69, .58, 0, -.19)
      const roof = new T.Shape().moveTo(-.63, .13).lineTo(0, .72).lineTo(.63, .13).closePath()
      const geometry = new T.ExtrudeGeometry(roof, { depth: .7, bevelEnabled: false }); geometry.translate(0, 0, -.35)
      group.add(new T.Mesh(geometry, face))
      p.box(.22, .36, .03, -.12, -.35, .31, accent)
      p.box(.17, .17, .03, .22, -.1, .31, accent)
      break
    }
    case 'wallet':
      p.plate(1.1, .78, .28)
      p.plate(.4, .3, .12, face, .05).position.set(.36, 0, .2)
      p.sphere(.055, .38, 0, .28, accent)
      p.box(.66, .035, .03, -.1, .24, .17, accent)
      break
    case 'coin': {
      const disk = new T.Mesh(new T.CylinderGeometry(.48, .48, .13, 40), face)
      disk.rotation.x = Math.PI / 2; group.add(disk)
      p.ring(.44, .027, accent).position.z = .09
      p.box(.055, .53, .035, 0, 0, .1, accent)
      p.line([[.17, .17, .11], [-.14, .17, .11], [-.14, 0, .11], [.14, 0, .11], [.14, -.17, .11], [-.17, -.17, .11]], accent, .026)
      break
    }
    case 'invoice':
      p.plate(.9, 1.25, .1, body, .04)
      for (const [i, width] of [.5, .58, .42, .58].entries()) p.box(width, .045, .03, (width - .58) / 2, .38 - i * .2, .08, i === 0 ? accent : face)
      p.box(.55, .075, .035, 0, -.45, .09, accent)
      break
    case 'chat': {
      p.plate(1.12, .82, .17)
      const tail = p.box(.25, .3, .15, -.3, -.45); tail.rotation.z = -.4
      for (const x of [-.3, 0, .3]) p.sphere(.065, x, 0, .13, accent)
      break
    }
    case 'users':
      for (const [index, x] of [-.43, 0, .43].entries()) {
        const y = index === 1 ? -.12 : .15
        p.sphere(.16, x, y + .3, index === 1 ? .15 : 0, accent)
        p.plate(.28, .48, .22, index === 1 ? face : body).position.set(x, y - .12, index === 1 ? .15 : 0)
      }
      break
    case 'shield': {
      const shape = new T.Shape().moveTo(0, .7).lineTo(.56, .45).lineTo(.5, -.15).quadraticCurveTo(.34, -.48, 0, -.69).quadraticCurveTo(-.34, -.48, -.5, -.15).lineTo(-.56, .45).closePath()
      const geometry = new T.ExtrudeGeometry(shape, { depth: .15, bevelEnabled: true, bevelSize: .035, bevelThickness: .035, bevelSegments: 2 }); geometry.translate(0, 0, -.08)
      group.add(new T.Mesh(geometry, face))
      p.line([[-.27, .04, .13], [-.06, -.17, .13], [.3, .23, .13]], accent, .045)
      break
    }
    case 'clock': {
      p.ring(.58, .065, face); p.ring(.48, .018, accent)
      for (let i = 0; i < 12; i++) {
        const angle = i / 12 * Math.PI * 2
        p.sphere(.024, Math.sin(angle) * .47, Math.cos(angle) * .47, .04, accent)
      }
      hand = new T.Group(); group.add(hand)
      const needle = new T.Mesh(new T.BoxGeometry(.04, .39, .05), accent); needle.position.y = .16; hand.add(needle)
      p.box(.27, .05, .06, .1, 0, .08, accent); p.sphere(.06, 0, 0, .1, accent)
      break
    }
    case 'cube':
      for (const [x, y, z] of [[-.26, -.24, .1], [.29, -.24, -.04], [0, .27, -.08]]) {
        const cube = p.box(.48, .48, .48, x, y, z, face)
        const edge = new T.LineSegments(new T.EdgesGeometry(cube.geometry), new T.LineBasicMaterial({ color: colors.accent, transparent: true, opacity: .5 }))
        cube.add(edge)
      }
      break
    case 'code':
      p.plate(1.25, .85, .16)
      p.line([[-.23, .22, .14], [-.45, 0, .14], [-.23, -.22, .14]], accent)
      p.line([[.23, .22, .14], [.45, 0, .14], [.23, -.22, .14]], accent)
      p.line([[.08, .25, .14], [-.08, -.25, .14]], accent)
      break
    case 'link': {
      const left = p.ring(.3, .075, accent); left.scale.y = 1.4; left.position.set(-.2, -.16, .05); left.rotation.z = -.65
      const right = p.ring(.3, .075, face); right.scale.y = 1.4; right.position.set(.2, .16, 0); right.rotation.z = -.65
      break
    }
    case 'download':
      p.plate(1, 1.15, .12)
      p.line([[0, .36, .14], [0, -.16, .14]], accent, .05)
      p.line([[-.23, .04, .14], [0, -.19, .14], [.23, .04, .14]], accent, .05)
      p.line([[-.32, -.33, .14], [-.32, -.42, .14], [.32, -.42, .14], [.32, -.33, .14]], accent, .035)
      break
  }
  group.rotation.set(.12, -.25, 0)
  return { group, body, accent, materials: [body, face, accent], hand }
}
