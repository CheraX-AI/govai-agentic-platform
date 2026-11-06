export default function UnavailablePage({ developmentOnly = false }: { developmentOnly?: boolean }) {
  return <main className="unknown-preview"><h1>{developmentOnly ? 'Development preview unavailable' : 'Preview unavailable'}</h1><p>{developmentOnly ? 'Static development previews are disabled in production.' : 'This destination is not implemented.'}</p><a href="#/login">Back to Login</a></main>
}
