import{r as _,k as v,l as i,m as R,e as b,E as V,a as t,o as d,b as D,h as j,n as y,d as m,w as k}from"./index-53a47fbc.js";const E={__name:"Render",props:{renderJson:String,formData:{type:Object,default:()=>({})}},emits:["replies"],setup(c,{emit:r}){const l=c,f=_(null),e=v({globalDsv:{API_SERV:"",HTTP:i},json:{},ready:!1,dialogVisible:!1,innerRender:"",params:{}});R(()=>{console.log("----- props -----",l);const a=b(),o=l.renderJson??"/model/"+a.meta.model+".json";i.get(o).then(n=>{console.log(n),e.json=n.data,e.ready=!0}).catch(n=>{console.log(n),V.error({offset:200,message:"获取页面渲染数据失败"})})});function s(a,o){if(console.log("---- replies ----",a,o),a=="close"){e.dialogVisible=!1,r("replies",a,o);return}e.params=o,e.innerRender=a,e.dialogVisible=!0,r("replies",a,o)}return(a,o)=>{const n=t("v-form-render"),p=t("Render",!0),u=t("el-dialog");return d(),D("div",null,[e.ready?(d(),j(n,{key:0,"form-json":e.json,"form-data":l.formData,"option-data":l.optionData,ref_key:"vFormRef",ref:f,onReplies:s,"global-dsv":e.globalDsv},null,8,["form-json","form-data","option-data","global-dsv"])):y("",!0),m(u,{modelValue:e.dialogVisible,"onUpdate:modelValue":o[0]||(o[0]=g=>e.dialogVisible=g),title:"数据窗口","destroy-on-close":"","align-center":""},{default:k(()=>[m(p,{renderJson:e.innerRender,onReplies:s,formData:e.params},null,8,["renderJson","formData"])]),_:1},8,["modelValue"])])}}};export{E as default};
