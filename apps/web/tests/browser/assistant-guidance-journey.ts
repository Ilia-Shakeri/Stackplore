import {readFileSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {expect,type Page} from '@playwright/test';
import {parseAssistantRunResponse} from '@stackplore/api-client';

// Production routes and the real local processor are used; no request is intercepted.
export async function assistantGuidanceJourney(page:Page,locale:'en'|'fa',role:'administrator'|'viewer') {
 const metadata={source_revision:process.env.GITHUB_SHA,locale,role,status:'failed',consent_sent:false,document_current:false,not_facts:false,witness_current:false,cleared:false,coverage_valid:false};
 const words=Object.assign({},JSON.parse(readFileSync(resolve(`src/features/assistant/messages.${locale}.json`),'utf8')) as Record<string,string>,JSON.parse(readFileSync(resolve(`src/features/assistant/runbooks.messages.${locale}.json`),'utf8')) as Record<string,string>);
 const label=(key:string)=>{const value=words[key];if(value===undefined)throw new Error('Guidance translation unavailable');return value;};
 const resourcePage=page.url();
 const guideTitle=locale==='en'?'Local check guide':'راهنمای بررسی محلی';
 const guideBody=locale==='en'?'For next checks only: compare current service health and recent events. This guide is not a live observation.':'فقط برای بررسی بعدی: سلامت کنونی سرویس و رخدادهای اخیر را مقایسه کنید. این راهنما مشاهدهٔ زنده نیست.';
 try {
  await page.getByRole('button',{name:label('assistant.open'),exact:true}).click();
  const panel=page.locator('.assistant');
  await panel.getByRole('button',{name:label('assistant.guide.choose'),exact:true}).click();
  await panel.getByRole('textbox',{name:label('runbooks.query'),exact:true}).fill(locale==='en'?'checks':'بررسی');
  await panel.getByRole('button',{name:label('runbooks.search'),exact:true}).click();
  await panel.getByRole('button',{name:guideTitle,exact:true}).click();
  await panel.getByRole('checkbox',{name:label('assistant.guide.consent'),exact:true}).check();
  await panel.getByRole('button',{name:label('assistant.guide.include'),exact:true}).click();
  await panel.getByRole('textbox',{name:label('assistant.question'),exact:true}).fill(locale==='en'?'Give one next check using the selected guide. Do not state any facts from the guide.':'با راهنمای منتخب فقط یک بررسی بعدی پیشنهاد کنید. متن راهنما را واقعیت مشاهده‌شده ندانید.');
  const admission=page.waitForResponse((response)=>response.request().method()==='POST' && new URL(response.url()).pathname==='/api/v1/assistant/runs');
  await panel.getByRole('button',{name:label('assistant.send'),exact:true}).click();
  const response=await admission;
  if(response.status()!==202)throw new Error('Guidance admission failed');
  const posted=response.request().postDataJSON() as {reviewed_document?:{consent?:unknown}};
  metadata.consent_sent=posted.reviewed_document?.consent===true;
  const receipt=await response.json() as {run?:{id?:string}};
  const id=receipt.run?.id;
  if(id===undefined || !/^ar_[a-f0-9]{64}$/u.test(id))throw new Error('Guidance run identity missing');
  try {await page.waitForFunction(()=>document.querySelectorAll('.assistant__turns .assistant__text').length>0,undefined,{timeout:110_000});} catch {throw new Error('Bounded guidance completion was not published');}
  const current=await page.request.get(`/api/v1/assistant/runs/${id}`);
  if(current.status()!==200)throw new Error('Guidance snapshot unavailable');
  const ready=parseAssistantRunResponse(await current.json(),id);
  if(ready.content_state!=='ready')throw new Error('Guidance response not ready');
  const guides=ready.citations.filter((citation)=>citation.kind==='document');
  const guide=guides[0];
  metadata.document_current=guides.length===1 && guide?.document_reference?.corpus_id==='browser_approved' && guide.document_reference.version===2 && guide.document_reference.document_id==='safe_check' && guide.document_reference.locale===locale;
  metadata.not_facts=ready.answer.sections.every((section)=>section.kind!=='facts' || !section.citations.includes(guide?.id??''));
  metadata.coverage_valid=ready.coverage?.length===2 && ready.coverage[0]?.tool==='inventory' && ready.coverage[1]?.tool==='reviewed_document' && ready.coverage.every((coverage)=>coverage.records===1);
  await panel.getByRole('button',{name:label('assistant.guide.witness'),exact:true}).click();
  const witness=panel.getByRole('article',{name:label('runbooks.document'),exact:true});
  await expect(witness).toBeVisible();
  metadata.witness_current=await witness.evaluate((element,body)=>element.textContent?.includes(body)===true,guideBody);
  await page.evaluate(()=>{window.dispatchEvent(new Event('pagehide'));});
  metadata.cleared=await page.evaluate((body)=>!document.body.textContent.includes(body),guideBody);
  for(const key of ['consent_sent','document_current','not_facts','coverage_valid','witness_current','cleared'] as const)expect(metadata[key]).toBe(true);
  metadata.status='passed';
 } finally {
  writeFileSync(resolve(`../../.tmp/hosted-guidance-${locale}-${role}.json`),`${JSON.stringify(metadata)}\n`,{mode:0o600});
  await page.goto(resourcePage);
 }
}
