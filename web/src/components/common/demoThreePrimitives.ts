import type { Group, Material, Mesh, MeshStandardMaterial } from 'three'

export type ThreeLibrary = typeof import('three')
export const demoThreePalette = { body: 0x1b3542, face: 0x284c59, accent: 0x78d4da, warning: 0xe5a28c, track: 0x29414c }

export interface DemoThreeObject {
  group: Group
  body: MeshStandardMaterial
  accent: MeshStandardMaterial
  materials: readonly Material[]
  hand?: Group
}

/** Small procedural solids: no downloaded models, images, fonts, or textures. */
export function demoThreePrimitives(T: ThreeLibrary, group: Group, body: Material) {
  function box(w: number, h: number, d: number, x = 0, y = 0, z = 0, material = body): Mesh {
    const mesh = new T.Mesh(new T.BoxGeometry(w, h, d), material)
    mesh.position.set(x, y, z); group.add(mesh)
    return mesh
  }
  function plate(w: number, h: number, d: number, material = body, radius = .11): Mesh {
    const shape = new T.Shape(), x = -w / 2, y = -h / 2, r = Math.min(radius, w / 3, h / 3)
    shape.moveTo(x + r, y); shape.lineTo(x + w - r, y); shape.quadraticCurveTo(x + w, y, x + w, y + r)
    shape.lineTo(x + w, y + h - r); shape.quadraticCurveTo(x + w, y + h, x + w - r, y + h)
    shape.lineTo(x + r, y + h); shape.quadraticCurveTo(x, y + h, x, y + h - r)
    shape.lineTo(x, y + r); shape.quadraticCurveTo(x, y, x + r, y)
    const geometry = new T.ExtrudeGeometry(shape, { depth: d, bevelEnabled: true, bevelSize: .025, bevelThickness: .025, bevelSegments: 2, curveSegments: 4 })
    geometry.translate(0, 0, -d / 2)
    const mesh = new T.Mesh(geometry, material); group.add(mesh)
    return mesh
  }
  function sphere(radius: number, x: number, y: number, z: number, material = body): Mesh {
    const mesh = new T.Mesh(new T.SphereGeometry(radius, 20, 12), material)
    mesh.position.set(x, y, z); group.add(mesh)
    return mesh
  }
  function ring(radius: number, thickness: number, material = body): Mesh {
    const mesh = new T.Mesh(new T.TorusGeometry(radius, thickness, 8, 48), material); group.add(mesh)
    return mesh
  }
  function line(points: readonly (readonly [number, number, number])[], material = body, thickness = .035): Mesh {
    const path = new T.CatmullRomCurve3(points.map(point => new T.Vector3(...point)), false, 'centripetal')
    const mesh = new T.Mesh(new T.TubeGeometry(path, Math.max(8, points.length * 8), thickness, 6, false), material)
    group.add(mesh); return mesh
  }
  return { box, plate, sphere, ring, line }
}

export function disposeDemoThreeGroup(group: Group, extraMaterials: readonly Material[] = []): void {
  const geometries = new Set<{ dispose: () => void }>(), materials = new Set<Material>(extraMaterials)
  group.traverse(object => {
    const mesh = object as Mesh
    if (mesh.geometry) geometries.add(mesh.geometry)
    if (mesh.material) for (const material of Array.isArray(mesh.material) ? mesh.material : [mesh.material]) materials.add(material)
  })
  for (const geometry of geometries) geometry.dispose()
  for (const material of materials) material.dispose()
  group.clear()
}
