import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import MediaCacheSettings, { type MediaCachePolicy } from './MediaCacheSettings'

const defaults: MediaCachePolicy = {
  mode:'automatic',storage:'ram',maxMiB:128,maxConcurrent:1,
  segmentSeconds:3,idleSeconds:90,cleanupGraceSeconds:15,
}
afterEach(()=>{cleanup();vi.unstubAllGlobals()})

it('loads defaults, allows manual disk selection and persists bounded settings',async()=>{
  const calls: {path:string;body?:unknown}[]=[]
  vi.stubGlobal('fetch',vi.fn(async (input:string,options?:RequestInit)=>{
    calls.push({path:input,body:options?.body && JSON.parse(String(options.body))})
    const policy = options?.method==='PUT' ?
      (JSON.parse(String(options.body)) as {policy:MediaCachePolicy}).policy : defaults
    return {ok:true,status:200,json:async()=>({
      policy,defaults,active:false,status:'Worker HLS ainda não ativo.',
    })}
  }))
  render(<MediaCacheSettings />)
  expect(await screen.findByLabelText('Gerenciamento de cache')).toBeTruthy()
  expect((screen.getByLabelText('Local do cache de vídeo') as HTMLSelectElement).disabled).toBe(true)
  fireEvent.change(screen.getByLabelText('Gerenciamento de cache'),{target:{value:'manual'}})
  fireEvent.change(screen.getByLabelText('Local do cache de vídeo'),{target:{value:'disk'}})
  fireEvent.change(screen.getByLabelText('Limite máximo de cache'),{target:{value:'512'}})
  fireEvent.change(screen.getByLabelText('Conversões simultâneas'),{target:{value:'2'}})
  fireEvent.click(screen.getByRole('button',{name:'Salvar configuração'}))
  await waitFor(()=>expect(calls).toHaveLength(2))
  expect(calls[1].path).toBe('/api/v1/admin/media/cache')
  expect(calls[1].body).toMatchObject({policy:{
    mode:'manual',storage:'disk',maxMiB:512,maxConcurrent:2,
  }})
  expect(await screen.findByText('Configuração salva no banco.')).toBeTruthy()
})

it('restores automatic default policy without implying the HLS worker is active',async()=>{
 vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,status:200,json:async()=>({
  policy:{...defaults,mode:'manual',storage:'disk',maxMiB:512},
  defaults,active:false,status:'Worker HLS ainda não ativo.',
 })})))
 render(<MediaCacheSettings/>)
 await screen.findByLabelText('Gerenciamento de cache')
 expect(screen.getByText('Worker HLS ainda não ativo.')).toBeTruthy()
 fireEvent.click(screen.getByRole('button',{name:'Restaurar padrões'}))
 expect((screen.getByLabelText('Gerenciamento de cache') as HTMLSelectElement).value).toBe('automatic')
 expect((screen.getByLabelText('Local do cache de vídeo') as HTMLSelectElement).disabled).toBe(true)
 expect(screen.getByText(/Clique em Salvar/)).toBeTruthy()
})
