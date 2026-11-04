import { createApp, h } from 'vue'
import { createRouter, createMemoryHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import App from '@nuxt/ui/components/App.vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import { featuresApi, type DatabaseTable, type DatabaseRow } from '@/api/features'
import { ApiError } from '@/api/http'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale } from '@/i18n'
import AdminDatabasePanel from '@/components/admin/AdminDatabasePanel.vue'
import '@/browser/styles.css'
const params=new URLSearchParams(location.search)
setExperience('browser');initializeBrowserTheme();setLocale(params.has('zh')?'zh-CN':'en')
const trace={writes:[] as unknown[]};Object.assign(window,{databaseAudit:trace})
const table:DatabaseTable={name:'operational_records',allowInsert:true,allowUpdate:true,allowDelete:true,highRisk:true,warning:'',columns:[
 {name:'id',declaredType:'text',nullable:false,primaryKeyPosition:1,editable:false,insertable:true,sensitive:false,protectionReason:'primaryKey'},
 {name:'label',declaredType:'text',nullable:false,primaryKeyPosition:0,editable:true,insertable:true,sensitive:false},
 {name:'amount',declaredType:'numeric(30,0)',nullable:false,primaryKeyPosition:0,editable:true,insertable:true,sensitive:false},
 {name:'updated_at',declaredType:'text',nullable:false,primaryKeyPosition:0,editable:false,insertable:false,sensitive:false,protectionReason:'generated'},
]}
let rows:DatabaseRow[]=params.has('empty')?[]:[{key:{id:'one'},recordHash:'before',values:{id:'one',label:'Operational label',amount:'900719925474099312345',updated_at:new Date().toISOString()}}]
async function pause(){if(params.has('slow'))await new Promise(resolve=>setTimeout(resolve,1600));if(params.has('error'))throw new ApiError(503,{code:'HTTP_ERROR',message:''})}
featuresApi.getDatabaseTables=async()=>{await pause();return{items:[table,{...table,name:'ledger_entries',allowInsert:false,allowUpdate:false,allowDelete:false,protectionReason:'financialEvidence',columns:table.columns.map(column=>({...column,editable:false,insertable:false,protectionReason:'financialEvidence'}))}]}}
featuresApi.queryDatabaseRows=async()=>{await pause();return{items:rows,nextCursor:null}}
featuresApi.reviewDatabaseMutation=async body=>{await pause();trace.writes.push(body);return{action:body.action,table:body.table,key:body.key??{},before:rows[0]?.values??null,after:body.action==='delete'?null:body.values,changedColumns:Object.keys(body.action==='delete' ? rows[0]?.values ?? {} : body.values ?? {}),reviewHash:'review',requiredConfirmation:`${body.action==='delete'?'DELETE':'EDIT'} ${body.table}`,rescueBackupRequired:true,warning:''}}
featuresApi.applyDatabaseMutation=async body=>{trace.writes.push(body);if(params.has('backup-error'))throw new ApiError(503,{code:'HTTP_ERROR',message:''});if(body.action==='delete'){rows=[];return{deleted:true,rescueBackupId:'verified-rescue'}}const row={key:{id:String(body.values.id??'one')},recordHash:'after',values:{...rows[0]?.values,...body.values}};rows=[row];return{row,rescueBackupId:'verified-rescue'}}
createApp({render:()=>h(App,{locale:params.has('zh')?zh_cn:en},()=>h('main',{class:'p-6 max-w-6xl mx-auto'},h(AdminDatabasePanel)))}).use(createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: { render: () => null } }] })).use(ui).mount('#app')
