export interface KnowledgeSource { labelKey: string; url: string }

/** Primary documentation backs general guidance; TX operating policies come from the project. */
export const knowledgeSources: Record<string, readonly KnowledgeSource[]> = {
  subscription: [
    { labelKey: 'knowledge.sourcesLabels.flclash', url: 'https://chen08209.github.io/FlClash/' },
    { labelKey: 'knowledge.sourcesLabels.v2rayn', url: 'https://github.com/2dust/v2rayN' },
    { labelKey: 'knowledge.sourcesLabels.v2rayng', url: 'https://github.com/2dust/v2rayNG' },
    { labelKey: 'knowledge.sourcesLabels.clientImport', url: 'https://github.com/2dust/v2rayN/wiki/Description-of-subscription' },
  ],
  networks: [
    { labelKey: 'knowledge.sourcesLabels.telecom', url: 'https://www.chinatelecom-ctap.com/global-ip-transit' },
    { labelKey: 'knowledge.sourcesLabels.unicom', url: 'https://kr.chinaunicomglobal.com/products-dia.php' },
    { labelKey: 'knowledge.sourcesLabels.mobile', url: 'https://iconnect.cmi.chinamobile.com/en/product/i-connect-data/ip-transit-dia' },
    { labelKey: 'knowledge.sourcesLabels.maxmind', url: 'https://support.maxmind.com/knowledge-base/articles/anonymizer-and-proxy-data-maxmind' },
  ],
  payment: [
    { labelKey: 'knowledge.sourcesLabels.bybit', url: 'https://www.bybit.com/en/help-center/article/FAQ-Crypto-Withdrawal' },
    { labelKey: 'knowledge.sourcesLabels.ethereum', url: 'https://ethereum.org/developers/docs/gas/' },
    { labelKey: 'knowledge.sourcesLabels.tron', url: 'https://developers.tron.network/docs/resource-model' },
    { labelKey: 'knowledge.sourcesLabels.solana', url: 'https://solana.com/docs/core/fees' },
    { labelKey: 'knowledge.sourcesLabels.aptos', url: 'https://aptos.dev/network/blockchain/gas-txn-fee' },
    { labelKey: 'knowledge.sourcesLabels.circle', url: 'https://help.circle.com/support/en/transfer-usdc-out-of-your-account?id=kb_article_view&sysparm_article=KB0010754' },
    { labelKey: 'knowledge.sourcesLabels.kraken', url: 'https://support.kraken.com/articles/how-to-withdraw-cryptocurrencies-from-your-kraken-account' },
  ],
  sharing: [
    { labelKey: 'knowledge.sourcesLabels.cgnat', url: 'https://blog.cloudflare.com/detecting-cgn-to-reduce-collateral-damage/' },
    { labelKey: 'knowledge.sourcesLabels.ietf', url: 'https://www.rfc-editor.org/rfc/rfc6269.html#section-13.1' },
    { labelKey: 'knowledge.sourcesLabels.detection', url: 'https://developers.cloudflare.com/bots/concepts/bot-score/' },
  ],
  rules: [],
  why: [
    { labelKey: 'knowledge.sourcesLabels.project', url: 'https://github.com/VanSail-Opensource/VanSail-Carpool' },
    { labelKey: 'knowledge.sourcesLabels.privacy', url: 'https://github.com/VanSail-Opensource/VanSail-Carpool/blob/main/internal/xnode/master_logs.go' },
  ],
}
