import { describe, expect, it } from 'vitest'
import type { RoutingPath } from '@/api/xnodeMember'
import { routingTree } from './routingTree'

const path = (nodes: string[], rules: string[]): RoutingPath => ({ nodes, rules, conditions: [], exclusions: [], excludedRules: [] })
const nodes = [{ id: 'a', name: 'Amsterdam' }, { id: 'b', name: 'Singapore' }]
const templates = [{ id: 'site', name: 'Work services', match: { domain: ['geosite:google'] } }]
const choices = [{ id: 'ab', inboundNodeId: 'a', outboundNodeId: 'b', templateId: 'site', inboundEndpointId: 'a-primary', outboundEndpointId: 'b-primary' }]
describe('routing tree', () => {
  it('keeps identifiers bounded across a long route chain', () => {
    const chain = Array.from({ length: 65 }, (_, index) => ({ id: String(index), name: String(index) }))
    const links = chain.slice(1).map((node, index) => ({ id: `rule-${index}`, inboundNodeId: chain[index]!.id, outboundNodeId: node.id, templateId: 'site' }))
    const tree = routingTree(chain, [path(chain.map(node => node.id), links.map(link => link.id))], links, [], templates, 'You', null)
    expect(Math.max(...tree.nodes.map(node => node.id.length))).toBeLessThan(2048)
  })
  it('keeps configured template leaves inspectable when preview is unavailable', () => {
    const tree = routingTree(nodes, [], choices, [], templates, 'You', null)
    expect(tree.nodes.at(-1)?.name).toBe('Work services')
    expect(tree.edges.some(edge => edge.highlighted)).toBe(false)
  })
  it('combines the user, shared node hops and administrator-named template leaves', () => {
    const paths = [path(['a', 'b'], ['ab'])]
    const tree = routingTree(nodes, paths, choices, [], templates, 'You', paths[0]!)
    expect(tree.nodes.map(node => node.name)).toEqual(['You', 'Amsterdam', 'Singapore', 'Work services'])
    expect(tree.nodes.at(-1)?.templateIds).toEqual(['site'])
    expect(tree.edges).toHaveLength(3)
    expect(tree.edges.every(edge => edge.highlighted)).toBe(true)
  })
  it('retains separate entry hosts without inventing a joined path', () => {
    const alternate = { ...choices[0]!, id: 'alternate', inboundEndpointId: 'a-alternate' }
    const tree = routingTree(nodes, [path(['a', 'b'], ['ab']), path(['a', 'b'], ['alternate'])], [...choices, alternate], [], templates, 'You', null)
    expect(tree.nodes.filter(node => node.nodeId === 'a')).toHaveLength(2)
    expect(tree.nodes.filter(node => node.templateIds)).toHaveLength(2)
    expect(tree.nodes.filter(node => node.id === 'root')).toHaveLength(1)
  })
  it('highlights only the complete traced branch, not a separate B to C entry', () => {
    const allNodes = [...nodes, { id: 'c', name: 'Tokyo' }]
    const rules = [...choices, { id: 'bc', inboundNodeId: 'b', outboundNodeId: 'c', templateId: 'site', inboundEndpointId: 'b-primary', outboundEndpointId: 'c-primary' }]
    const full = path(['a', 'b', 'c'], ['ab', 'bc'])
    const tree = routingTree(allNodes, [full, path(['b', 'c'], ['bc'])], rules, [], templates, 'You', full)
    expect(tree.nodes.filter(node => node.nodeId === 'b')).toHaveLength(2)
    expect(tree.nodes.filter(node => node.nodeId === 'b' && node.highlighted)).toHaveLength(1)
    expect(tree.nodes.filter(node => node.nodeId === 'c' && node.highlighted)).toHaveLength(1)
    expect(tree.edges.filter(edge => edge.highlighted)).toHaveLength(4)
    expect(tree.edges.filter(edge => !edge.highlighted)).toHaveLength(3)
  })
  it('shares the ingress and branches ordered rule sequences even when destinations match', () => {
    const other = { ...choices[0]!, id: 'other-rule' }
    const full = path(['a', 'b'], ['ab'])
    const tree = routingTree(nodes, [full, path(['a', 'b'], ['other-rule'])], [...choices, other], [], templates, 'You', full)
    expect(tree.nodes.filter(node => node.nodeId === 'a')).toHaveLength(1)
    expect(tree.edges.filter(edge => edge.from === 1)).toHaveLength(1)
    expect(tree.nodes.filter(node => node.nodeId === 'b')).toHaveLength(2)
    expect(tree.nodes.filter(node => node.nodeId === 'b' && node.highlighted)).toHaveLength(1)
  })
})
