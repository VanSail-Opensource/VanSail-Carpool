// Credential names verified against X-Node v2.0.6 (fa867d98), dnsproviders registry.
export const providerCatalog: { id: string; aliases: string[]; fields: string[] }[] = [
  {"id": "alidns", "aliases": ["aliyun"], "fields": ["ALICLOUD_ACCESS_KEY_ID", "ALICLOUD_ACCESS_KEY_SECRET"]},
  {"id": "azure", "aliases": [], "fields": ["AZURE_SUBSCRIPTION_ID", "AZURE_RESOURCE_GROUP_NAME", "AZURE_TENANT_ID", "AZURE_CLIENT_ID", "AZURE_CLIENT_SECRET"]},
  {"id": "bunny", "aliases": [], "fields": ["BUNNY_ACCESS_KEY"]},
  {"id": "cloudflare", "aliases": ["cf"], "fields": ["CLOUDFLARE_DNS_API_TOKEN"]},
  {"id": "desec", "aliases": [], "fields": ["DESEC_TOKEN"]},
  {"id": "digitalocean", "aliases": ["do"], "fields": ["DO_AUTH_TOKEN"]},
  {"id": "duckdns", "aliases": [], "fields": ["DUCKDNS_TOKEN", "DUCKDNS_OVERRIDE_DOMAIN"]},
  {"id": "gandi", "aliases": [], "fields": ["GANDI_BEARER_TOKEN"]},
  {"id": "godaddy", "aliases": [], "fields": ["GODADDY_API_TOKEN", "GODADDY_API_KEY", "GODADDY_API_SECRET"]},
  {"id": "googleclouddns", "aliases": ["gcp", "gcloud"], "fields": ["GCE_PROJECT", "GCE_SERVICE_ACCOUNT_FILE"]},
  {"id": "hetzner", "aliases": [], "fields": ["HETZNER_API_TOKEN"]},
  {"id": "huaweicloud", "aliases": ["huawei"], "fields": ["HUAWEICLOUD_ACCESS_KEY_ID", "HUAWEICLOUD_SECRET_ACCESS_KEY", "HUAWEICLOUD_REGION_ID"]},
  {"id": "linode", "aliases": [], "fields": ["LINODE_TOKEN"]},
  {"id": "namecheap", "aliases": [], "fields": ["NAMECHEAP_API_KEY", "NAMECHEAP_API_USER", "NAMECHEAP_API_ENDPOINT", "NAMECHEAP_CLIENT_IP"]},
  {"id": "namesilo", "aliases": [], "fields": ["NAMESILO_API_TOKEN"]},
  {"id": "netlify", "aliases": [], "fields": ["NETLIFY_PERSONAL_ACCESS_TOKEN"]},
  {"id": "ovh", "aliases": [], "fields": ["OVH_APPLICATION_KEY", "OVH_APPLICATION_SECRET", "OVH_CONSUMER_KEY", "OVH_ENDPOINT"]},
  {"id": "porkbun", "aliases": [], "fields": ["PORKBUN_API_KEY", "PORKBUN_API_SECRET_KEY"]},
  {"id": "route53", "aliases": ["aws"], "fields": ["AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_REGION", "AWS_PROFILE", "AWS_HOSTED_ZONE_ID"]},
  {"id": "tencentcloud", "aliases": ["tencent"], "fields": ["TENCENTCLOUD_SECRET_ID", "TENCENTCLOUD_SECRET_KEY", "TENCENTCLOUD_REGION"]},
  {"id": "vultr", "aliases": [], "fields": ["VULTR_API_TOKEN"]}
]
