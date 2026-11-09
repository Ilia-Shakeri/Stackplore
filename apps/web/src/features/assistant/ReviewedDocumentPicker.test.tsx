import {render,screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe,expect,it,vi} from 'vitest';
import {LocaleProvider} from '../../i18n';
import {controlledConversationApi} from './conversation.test-fixtures';
import {ReviewedDocumentPicker} from './ReviewedDocumentPicker';

describe('explicit reviewed guide selection',()=>{
 it('requires separate operator scope and user consent before including a bounded excerpt',async()=>{
  const controlled=controlledConversationApi();const owner=controlled.api.getOwner();if(owner===null)throw new Error('Missing owner');
  const reference={corpus_id:'ops',version:2,digest:'a'.repeat(64),document_id:'guide',locale:'en' as const,generation:0};
  controlled.api.runbookSearch.mockResolvedValue({hits:[{reference,title:'Private guide',snippet:'Safe snippet'}],next_cursor:''});
  controlled.api.runbookDocument.mockResolvedValue({document:{id:'guide',locale:'en',title:'Private guide title',body:'ظ'.repeat(200)},provenance:{kind:'operator_authored',source:'Synthetic fixture',license:'Fixture permission',use_reviewed:true,assistance_reviewed:true}});
  const onSelect=vi.fn();
  const view=render(<LocaleProvider defaultLocale="en"><ReviewedDocumentPicker api={controlled.api} owner={owner} selected={null} disabled={false} onSelect={onSelect}/></LocaleProvider>);
  const user=userEvent.setup();await user.click(screen.getByRole('button',{name:'Choose a guide'}));
  await user.type(screen.getByRole('textbox',{name:'Search in the current language'}),'guide');
  await user.click(screen.getByRole('button',{name:'Search documents'}));
  await user.click(await screen.findByRole('button',{name:'Private guide'}));
  const include=await screen.findByRole('button',{name:'Use this excerpt'});
  expect((include as HTMLButtonElement).disabled).toBe(true);
  expect(onSelect).not.toHaveBeenCalled();
  await user.click(screen.getByRole('checkbox',{name:'Include this excerpt in my local answer.'}));
  await user.click(include);
  const chosen=onSelect.mock.calls[0]?.[0] as {reference:unknown;excerpt:string}|undefined;
  expect(chosen?.reference).toEqual(reference);
  expect(new TextEncoder().encode(chosen?.excerpt).length).toBe(256);
  expect(chosen?.excerpt).toBe('ظ'.repeat(128));
  view.unmount();
 });
});
