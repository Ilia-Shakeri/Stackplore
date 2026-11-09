import {describe,expect,it} from 'vitest';
import {parseAssistantRunResponse} from './assistant-response';
import {assistantReadyFixture,assistantRunId} from './assistant-fixture.test-support';

describe('selected guide answer boundary',()=>{
 it('requires exact coverage and keeps guidance out of facts',()=>{
  const base=assistantReadyFixture();const resource=base.citations[0];if(resource===undefined)throw new Error('Missing fixture');
  const reference={corpus_id:'ops',version:2,digest:'a'.repeat(64),document_id:'guide',locale:'en',generation:0};
  const guide={...resource,id:`ev_${'d'.repeat(64)}`,kind:'document',document_reference:reference};
  const ready={...base,citations:[resource,guide],coverage:['inventory','reviewed_document'].map((tool)=>({tool,availability:'available',completeness:'complete',records:1,more_available:false})),answer:{sections:[...base.answer.sections,{kind:'next_checks',text:'Review the selected guidance.',citations:[guide.id]}]}};
  const parsed=parseAssistantRunResponse(ready,assistantRunId);
  expect(parsed.content_state).toBe('ready');
  expect(()=>parseAssistantRunResponse({...ready,answer:{sections:[{kind:'facts',text:'A guide is not observed service health.',citations:[guide.id]}]}},assistantRunId)).toThrow();
  expect(()=>parseAssistantRunResponse({...ready,coverage:ready.coverage.slice(0,1)},assistantRunId)).toThrow();
  expect(()=>parseAssistantRunResponse({...ready,citations:[resource,{...guide,document_reference:{...reference,href:'https://unapproved.invalid'}}]},assistantRunId)).toThrow();
 });
});
