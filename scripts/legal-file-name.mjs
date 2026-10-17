export const isLegalMaterialName = (name) =>
  /^(?:licen[sc]e|copying|notice|copyright|patents|ofl)(?:[._-].*)?$/iu.test(name) &&
  !/\.(?:[cm]?[jt]sx?|json|map|css|go|wasm)$/iu.test(name);
