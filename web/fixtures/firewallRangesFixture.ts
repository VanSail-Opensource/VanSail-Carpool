import type { XNodeSettings } from '@/api/xnode'

export function firewallRangesFixture(): NonNullable<XNodeSettings['firewall']> {
  return {
    enabled: true, protect: false,
    tcp_ports: [22, 443, 444], tcp_port_ranges: [{ from: 443, to: 445 }, { from: 10000, to: 10100 }],
    udp_ports: [443], udp_port_ranges: [{ from: 20000, to: 30000 }],
    ssh_ports: [22], ssh_port_ranges: [{ from: 2220, to: 2225 }],
    allow_prefixes: ['192.0.2.0/24', '2001:db8::/32'],
    forward: [
      { port: 20000, port_end: 20010, target_ip: '192.0.2.10', target_port: 443, network: 'udp' },
      { port: 30000, port_end: 30010, target_ip: '2001:db8::10', target_port: 40000, target_port_end: 40010, network: 'udp' },
      { listen_ip: '192.0.2.20', port: 8443, target_ip: '192.0.2.30', target_port: 443, network: 'tcp' },
    ],
  }
}
