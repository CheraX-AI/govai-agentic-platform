export function Brand({ showSubtitle = true, emblem = '/assets/figma/login/govai-emblem.png' }: { showSubtitle?: boolean; emblem?: string }) {
  return <div className="brand">
    <img src={emblem} width="57" height="48" alt="" />
    <div><span className="wordmark"><span>Gov</span><span>AI</span></span>{showSubtitle && <p>Sri Lanka Government Services</p>}</div>
  </div>
}
