declare module "virtual:vl-documents" {
  const documents: Record<string, import("./docs-paths").RenderedDocument>
  export default documents
}
