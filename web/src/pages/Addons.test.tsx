import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import type { Server } from '../types';
import { Addons } from './Addons';

const server = { id:'server-1',nodeId:'node-1',ownerUserId:'user-1',ownerUsername:'admin',name:'Paper',runtime:'paper',version:'1.21.4',javaVersion:21,memoryMb:2048,cpu:2,diskMb:10240,bindIp:'0.0.0.0',port:25565,desiredState:'offline',state:'offline',config:{},lastError:null,createdAt:'2026-01-01T00:00:00Z',updatedAt:'2026-01-01T00:00:00Z' } satisfies Server;

describe('Addons', () => {
  it('searches the selected provider with the server-scoped endpoint', async () => {
    const fetchMock=vi.fn()
      .mockResolvedValueOnce(new Response('[]',{status:200,headers:{'Content-Type':'application/json'}}))
      .mockResolvedValueOnce(new Response(JSON.stringify({items:[{provider:'modrinth',projectId:'skinrestorer',name:'SkinsRestorer',description:'Restore skins',iconUrl:'',downloads:1000}]}),{status:200,headers:{'Content-Type':'application/json'}}));
    vi.stubGlobal('fetch',fetchMock);
    render(<I18nProvider><Addons server={server} notify={() => undefined} onError={() => undefined} /></I18nProvider>);
    await screen.findByText('Belum ada add-on terkelola');
    fireEvent.change(screen.getByPlaceholderText('Cari plugin atau mod…'),{target:{value:'skins'}});
    fireEvent.click(screen.getByRole('button',{name:'Cari'}));
    expect(await screen.findByText('SkinsRestorer')).toBeInTheDocument();
    await waitFor(() => expect(String(fetchMock.mock.calls[1][0])).toContain('/addons/search?provider=modrinth&q=skins'));
  });
});
