import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import CatalogProviderGallery from './CatalogProviderGallery'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

it('shows curated providers without exposing upstream URLs', async () => {
  const calls: string[] = []
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    calls.push(url)
    return {ok:true,status:200,json:async()=>({providers:[
      {id:'tmdb',name:'The Movie Database',description:'Filmes e séries',category:'Filmes e séries',ready:true,installed:false},
      {id:'anime-catalogs',name:'Anime Catalogs',description:'Metadados',category:'Animes',ready:false,installed:false,note:'Configuração necessária'},
    ]})}
  }))
  render(<CatalogProviderGallery onInstalled={()=>{}} />)
  expect(await screen.findByRole('heading',{name:'The Movie Database'})).toBeTruthy()
  expect(screen.getByRole('button',{name:'Em preparação'}).hasAttribute('disabled')).toBe(true)
  expect(screen.queryByText(/baby-beamup\.club/)).toBeNull()
  fireEvent.click(screen.getByRole('button',{name:'Animes'}))
  expect(screen.queryByRole('heading',{name:'The Movie Database'})).toBeNull()
  expect(calls).toEqual(['/api/v1/admin/catalog/providers'])
})

it('installs a curated provider using only its safe ID', async () => {
  const calls: string[]=[]
  vi.stubGlobal('fetch',vi.fn(async(url:string,opts?:RequestInit)=>{
    calls.push(url)
    if(opts?.method==='POST')return {ok:true,status:201,json:async()=>({source:{
      id:'123e4567-e89b-12d3-a456-426614174000',name:'The Movie Database',enabled:true,
      cacheMode:'on-demand',catalogs:[],
    }})}
    return {ok:true,status:200,json:async()=>({providers:[
      {id:'tmdb',name:'The Movie Database',description:'Filmes e séries',category:'Filmes e séries',ready:true,installed:false},
    ]})}
  }))
  const onInstalled=vi.fn()
  render(<CatalogProviderGallery onInstalled={onInstalled}/>)
  fireEvent.click(await screen.findByRole('button',{name:'Adicionar'}))
  await waitFor(()=>expect(onInstalled).toHaveBeenCalledOnce())
  expect(calls[1]).toBe('/api/v1/admin/catalog/providers/tmdb')
  expect(screen.getByRole('button',{name:'Conectado'}).hasAttribute('disabled')).toBe(true)
})
