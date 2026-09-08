import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import type { Server } from '../types';
import { Modpacks } from './Modpacks';

const server = { id:'server-1',nodeId:'node-1',ownerUserId:'user-1',ownerUsername:'admin',name:'BocahSMP',runtime:'paper',version:'1.21.4',javaVersion:21,memoryMb:4096,cpu:2,diskMb:10240,bindIp:'0.0.0.0',port:25565,desiredState:'running',state:'running',config:{},lastError:null,createdAt:'2026-01-01T00:00:00Z',updatedAt:'2026-01-01T00:00:00Z' } satisfies Server;

describe('Modpacks', () => {
  it('loads CurseForge versions and requires the exact server name before replacement', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({item:null}),{status:200,headers:{'Content-Type':'application/json'}}))
      .mockResolvedValueOnce(new Response(JSON.stringify({items:[{projectId:'1148445',slug:'all-the-mods-11',name:'All the Mods 11',summary:'A large pack',iconUrl:'',downloads:1000}]}),{status:200,headers:{'Content-Type':'application/json'}}))
      .mockResolvedValueOnce(new Response(JSON.stringify({items:[{fileId:'8171764',name:'ATM11 0.0.22',fileName:'atm11.zip',runtime:'neoforge',minecraftVersion:'26.1.1',javaVersion:25,releaseType:'beta',publishedAt:'2026-06-01T00:00:00Z'}]}),{status:200,headers:{'Content-Type':'application/json'}}))
      .mockResolvedValueOnce(new Response(JSON.stringify({job:{id:'job-1'}}),{status:202,headers:{'Content-Type':'application/json'}}))
      .mockResolvedValueOnce(new Response(JSON.stringify({item:null}),{status:200,headers:{'Content-Type':'application/json'}}));
    vi.stubGlobal('fetch', fetchMock);
    const reload = vi.fn().mockResolvedValue(undefined);
    render(<I18nProvider><Modpacks server={server} reload={reload} notify={() => undefined} onError={() => undefined} /></I18nProvider>);

    await screen.findByText('Belum menggunakan modpack');
    fireEvent.change(screen.getByPlaceholderText('Cari modpack, misalnya All the Mods…'), {target:{value:'all the mods'}});
    fireEvent.click(screen.getByRole('button',{name:'Cari'}));
    fireEvent.click(await screen.findByRole('button',{name:/All the Mods 11/}));
    expect(await screen.findByText(/neoforge · Minecraft 26.1.1 · Java 25/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button',{name:'Pasang modpack'}));

    const confirmButton = screen.getByRole('button',{name:'Backup dan pasang'});
    expect(confirmButton).toBeDisabled();
    fireEvent.change(screen.getByLabelText('Ketik “BocahSMP” untuk melanjutkan'), {target:{value:'BocahSMP'}});
    expect(confirmButton).toBeEnabled();
    fireEvent.click(confirmButton);

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(5));
    const installCall = fetchMock.mock.calls[3];
    expect(String(installCall[0])).toContain('/api/v1/servers/server-1/modpacks');
    expect(JSON.parse(String(installCall[1]?.body))).toEqual({projectId:'1148445',fileId:'8171764',confirmation:'BocahSMP'});
    expect(reload).toHaveBeenCalled();
  });
});
