// Only new listeners use this preset. Imported methods and keys remain explicit.
export function shadowsocksConfiguration(): Record<string, unknown> {
  const key = globalThis.crypto.getRandomValues(new Uint8Array(32))
  return {
    inbounds: [{ tag: 'ss2022', listen: '0.0.0.0', port: 443, protocol: 'shadowsocks', settings: { method: '2022-blake3-aes-256-gcm', password: globalThis.btoa(String.fromCharCode(...key)), network: 'tcp,udp' } }],
    outbounds: [{ tag: 'direct', protocol: 'freedom' }, { UserOutbound: {} }],
    routing: { rules: [{ UserRouting: {} }] },
  }
}
