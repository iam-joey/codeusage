// codeusage — iOS Large widget: all ranges + per-machine last-seen. No stacking.
const BURN_URL = "https://burn.iamdsv.dev";
let   BURN_TOKEN = "REPLACE_WITH_YOUR_TOKEN";
if (args.widgetParameter) BURN_TOKEN = String(args.widgetParameter).trim();

const INK=new Color("#f4f5f2"), DIM=new Color("#8a918b"), DIM2=new Color("#6a726b");
const AMBER=new Color("#ffb020"), AMBER2=new Color("#ff8a3c"), SOFT=new Color("#ffd39a"), UP=new Color("#4ad884"), DN=new Color("#ff5c5c");
const RANGES=[["today","TODAY"],["7d","7 DAYS"],["30d","30 DAYS"]];

const usd=n=>{ n=Number(n)||0;
  if(n>=1e6) return "$"+(n/1e6).toFixed(1)+"M";
  if(n>=1e4) return "$"+(n/1e3).toFixed(1)+"K";
  if(n>=1e3) return "$"+Math.round(n).toLocaleString("en-US");
  return "$"+n.toFixed(2); };
const tok=n=>{ n=Number(n)||0;
  if(n>=1e12) return (n/1e12).toFixed(2)+"T";
  if(n>=1e9){ const v=n/1e9; return (v>=100?Math.round(v):v.toFixed(2))+"B"; }
  if(n>=1e6){ const v=n/1e6; return (v>=100?Math.round(v):v.toFixed(1))+"M"; }
  if(n>=1e3) return Math.round(n/1e3)+"K";
  return String(n); };
function relTime(iso){
  if(!iso) return "";
  const t=Date.parse(iso); if(isNaN(t)) return "";
  const s=Math.max(0,(Date.now()-t)/1000);
  if(s<60) return "just now";
  if(s<3600) return Math.floor(s/60)+"m ago";
  if(s<86400) return Math.floor(s/3600)+"h ago";
  return Math.floor(s/86400)+"d ago";
}

async function getStats(){
  const r=new Request(BURN_URL.replace(/\/$/,"")+"/stats");
  r.headers={Authorization:"Bearer "+BURN_TOKEN}; r.timeoutInterval=15;
  return await r.loadJSON();
}
function sparkImage(vals){
  const w=600,h=150,padY=16,padX=16,dotR=7;
  const ctx=new DrawContext(); ctx.size=new Size(w,h); ctx.opaque=false; ctx.respectScreenScale=true;
  const mx=Math.max(...vals),mn=Math.min(...vals),rg=(mx-mn)||1;
  const x0=padX, x1=w-padX-dotR, st=(x1-x0)/(vals.length-1);
  const pts=vals.map((v,i)=>new Point(x0+i*st, h-padY-((v-mn)/rg)*(h-2*padY)));
  const a=new Path(); a.move(new Point(x0,h)); pts.forEach(p=>a.addLine(p)); a.addLine(new Point(x1,h)); a.closeSubpath();
  ctx.setFillColor(new Color("#ffb020",0.16)); ctx.addPath(a); ctx.fillPath();
  const l=new Path(); l.move(pts[0]); for(let i=1;i<pts.length;i++) l.addLine(pts[i]);
  ctx.setStrokeColor(AMBER); ctx.setLineWidth(5); ctx.addPath(l); ctx.strokePath();
  const e=pts[pts.length-1]; ctx.setFillColor(AMBER2); ctx.fillEllipse(new Rect(e.x-dotR,e.y-dotR,2*dotR,2*dotR));
  return ctx.getImage();
}
async function build(){
  const w=new ListWidget(); w.setPadding(17,18,15,18);
  const g=new LinearGradient(); g.colors=[new Color("#15181c"),new Color("#0e1113")]; g.locations=[0,1]; w.backgroundGradient=g;
  w.url=BURN_URL+"/#t="+BURN_TOKEN;
  let data;
  try{ data=await getStats(); }catch(e){
    const t=w.addText("codeusage"); t.font=Font.semiboldSystemFont(15); t.textColor=INK;
    w.addSpacer(4); const m=w.addText("can't reach server — check token"); m.font=Font.systemFont(12); m.textColor=DIM; return w;
  }
  const R=data.ranges||{};
  const machines=(data.machines||[]).map(m=>({name:m.machine,on:!m.stale,toks:(m["30d"]||{}).tokens||0,seen:relTime(m.updated_at)})).sort((a,b)=>b.toks-a.toks);
  const spark=(data.spark||[]).map(p=>Number(p.tokens)||0);

  const head=w.addStack(); head.centerAlignContent();
  try{ const s=SFSymbol.named("flame.fill"); s.applyFont(Font.systemFont(15)); const im=head.addImage(s.image); im.imageSize=new Size(16,16); im.tintColor=AMBER2; head.addSpacer(7);}catch(e){}
  const nm=head.addText("codeusage"); nm.font=Font.semiboldSystemFont(14); nm.textColor=INK;

  w.addSpacer(13);
  const row=w.addStack();
  RANGES.forEach((rr,i)=>{
    if(i>0) row.addSpacer();
    const r=R[rr[0]]||{cost_usd:0,tokens:0,change_pct:null};
    const c=row.addStack(); c.layoutVertically();
    const lab=c.addText(rr[1]); lab.font=Font.mediumSystemFont(9.5); lab.textColor=DIM;
    c.addSpacer(3);
    const val=c.addText(tok(r.tokens)); val.font=Font.boldSystemFont(19); val.textColor=INK;
    c.addSpacer(3);
    const tk=c.addText(usd(r.cost_usd)); tk.font=Font.mediumSystemFont(10); tk.textColor=SOFT;
    c.addSpacer(2);
    if(r.change_pct!==null&&r.change_pct!==undefined){
      const up=r.change_pct>=0;
      const t=c.addText((up?"▲ ":"▼ ")+Math.abs(r.change_pct)+"%"); t.font=Font.semiboldSystemFont(10.5); t.textColor=up?UP:DN;
    } else { const t=c.addText("—"); t.font=Font.systemFont(10.5); t.textColor=DIM2; }
  });

  w.addSpacer(13);
  if(spark.length>=2){ try{ const gw=w.addStack(); gw.addSpacer(); const img=gw.addImage(sparkImage(spark)); img.imageSize=new Size(290,80); gw.addSpacer(); }catch(e){} }

  w.addSpacer(13);
  const ml=w.addText("MACHINES · 30D"); ml.font=Font.mediumSystemFont(9.5); ml.textColor=DIM;
  w.addSpacer(7);
  const top=machines.slice(0,3), hid=machines.slice(3);
  for(const m of top){
    const rw=w.addStack(); rw.centerAlignContent();
    const d=rw.addText("●"); d.font=Font.systemFont(9); d.textColor=m.on?UP:DN; rw.addSpacer(9);
    const n=rw.addText(m.name); n.font=Font.systemFont(13.5); n.textColor=m.on?INK:DN; rw.addSpacer();
    const sn=rw.addText(m.seen); sn.font=Font.systemFont(10.5); sn.textColor=m.on?DIM2:DN; rw.addSpacer(8);
    const v=rw.addText(tok(m.toks)); v.font=Font.semiboldSystemFont(13.5); v.textColor=m.on?INK:DIM2;
    w.addSpacer(9);
  }
  if(hid.length){
    const rest=hid.reduce((s,m)=>s+m.toks,0), inact=hid.filter(m=>!m.on).length;
    const rw=w.addStack(); const l=rw.addText("+"+hid.length+" more"+(inact?" · "+inact+" inactive":"")); l.font=Font.systemFont(11.5); l.textColor=inact?DN:DIM; rw.addSpacer();
    const v=rw.addText(tok(rest)); v.font=Font.systemFont(11.5); v.textColor=DIM;
  }
  w.refreshAfterDate=new Date(Date.now()+1200*1000);
  return w;
}
const widget=await build();
if(config.runsInWidget) Script.setWidget(widget);
else widget.presentLarge();
Script.complete();
