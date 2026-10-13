import { createApp, defineComponent, h, shallowRef } from 'vue'
import { createRouter, createMemoryHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import UApp from '@nuxt/ui/components/App.vue'
import AdminUSDTPage from './src/components/admin/usdt/AdminUSDTPage.vue'
import CryptoPaymentInstructions from './src/components/billing/CryptoPaymentInstructions.vue'
import StatisticsPaymentDonut from './src/components/statistics/StatisticsPaymentDonut.vue'
import { t } from './src/i18n'
import './src/styles/main.css'

const params = new URLSearchParams(location.search)
const mode = params.get('state') ?? 'populated'
const display = params.get('view') ?? 'admin'
const networks = ['tron','ethereum','bsc','polygon','arbitrum','xlayer','plasma','solana','aptos','ton']
const endpoint = { id:'node',rpcUrl:'',wsUrl:'',indexerUrl:'',apiKey:'',useWs:true,rpcConfigured:true,wsConfigured:true,indexerConfigured:false,keyConfigured:true,host:'rpc.example' }
const config = { enabled:true,name:'USDT',rate:'10',expiryMinutes:30,concurrency:2,networks:networks.map(network=>({network,enabled:network==='ethereum',addresses:network==='ethereum'?['0x1111111111111111111111111111111111111111']:[],endpoints:network==='ethereum'?[{...endpoint}]:[]})) }
const transfer = { id:'a'.repeat(64),network:'ethereum',token:'0x'+'a'.repeat(40),recipient:'0x'+'1'.repeat(40),atomicAmount:'2001000',amount:'2.001000',txHash:'0x'+'b'.repeat(64),eventId:'0',occurredAt:new Date().toISOString(),orderId:'payment-order',status:'review',reason:'USDT_LATE_PAYMENT' }
const calls: Array<{path:string;body:unknown}> = []
Object.assign(window, { auditCalls:calls })
window.fetch = async (input, options) => {
  const path = String(input)
  const body = options?.body ? JSON.parse(String(options.body)) : undefined
  calls.push({path,body})
  if (mode==='loading') await new Promise(resolve=>setTimeout(resolve,12000))
  if (mode==='error') return new Response(JSON.stringify({code:'USDT_RPC_UNAVAILABLE',message:'fixture'}),{status:502,headers:{'Content-Type':'application/json'}})
  let result: unknown = {}
  if (path.endsWith('/status')) result={networks:networks.map(network=>({network,state:network==='ethereum'?(mode==='rpc-error'?'error':'ready'):'disabled',cursor:network==='ethereum'?'21004567':'',updatedAt:new Date().toISOString(),errorCode:network==='ethereum'&&mode==='rpc-error'?'USDT_RPC_UNAVAILABLE':'',connection:network==='ethereum'?'websocket':'none'}))}
  else if (path.includes('/review/') && path.endsWith('/resolution')) result={orderId:'payment-order',transferId:transfer.id,credited:true,replayed:false}
  else if (path.includes('/review')) result={items:mode==='empty'?[]:[transfer],nextCursor:''}
  else if (path.endsWith('/check')) result={network:body.network,success:true,errorCode:''}
  else if (path.endsWith('/usdt')) result=options?.method==='PUT'?{...body,networks:body.networks.map((n:any)=>({...n,endpoints:n.endpoints.map((e:any)=>({...e,rpcUrl:'',wsUrl:'',apiKey:'',rpcConfigured:true,keyConfigured:true}))}))}:config
  return new Response(JSON.stringify(result),{status:200,headers:{'Content-Type':'application/json'}})
}
const qr = shallowRef<string|null>(null)
const order = { id:'payment-order',provider:'builtin_usdt',methodId:'builtin_usdt:usdt.erc20',providerRail:'usdt.erc20',status:'pending',txb:{currency:'TXB',minor:'2000',display:'20 TXB'},payableAmount:'2.000',payableCurrency:'USDT',rateSnapshot:'10',rateDirection:'txb_per_currency',paymentUrl:null,qrPayload:null,receivingAddress:'0x'+'1'.repeat(40),actualCryptoAmount:'2.001',actualCryptoCurrency:'USDT',expiresAt:new Date(Date.now()+(mode==='expired'?-1000:1800000)).toISOString(),paidAt:null,refundedAt:null,cancelledAt:null,cancelReason:'',providerCancelStatus:'',createdAt:new Date().toISOString(),updatedAt:new Date().toISOString() }
if (display==='payment') import('qrcode').then(async module=>{qr.value=await module.default.toDataURL(order.receivingAddress)})
const root=defineComponent({setup:()=>()=>h(UApp,null,{default:()=>h('div',{class:'page page--admin',style:'padding:16px;max-width:1050px;margin:auto'},display==='payment'?h(CryptoPaymentInstructions,{order:order as any,stage:'pending',qrDataUrl:qr.value,error:null,canRecreate:true}):display==='statistics'?h(StatisticsPaymentDonut,{items:[{id:'ezpay:paid',label:'',value:2},{id:'bepusdt:paid',label:'',value:3},{id:'builtin_usdt:paid',label:'',value:4}]}):h(AdminUSDTPage))})})
const router=createRouter({history:createMemoryHistory(),routes:[{path:'/',component:root}]})
const app=createApp(root);app.config.globalProperties.$t=t;app.use(router);app.use(ui);router.isReady().then(()=>app.mount('#app'))
