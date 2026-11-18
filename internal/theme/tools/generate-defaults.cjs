// Extract only vl's supported color/default graph from a pinned VS Code
// source checkout. Requires TypeScript for development, never at runtime.
// NODE_PATH=/path/to/node_modules bun internal/theme/tools/generate-defaults.cjs VSCODE_SOURCE GIT_PACKAGE_JSON
const ts = require('typescript'), fs = require('fs'), path = require('path');
const root = process.argv[2], gitPackage = process.argv[3] || path.join(root, 'extensions/git/package.json');
const paths = [
    ...fs.readdirSync(path.join(root, 'src/vs/platform/theme/common/colors')).filter(x => x.endsWith('.ts')).map(x => 'src/vs/platform/theme/common/colors/' + x),
    'src/vs/workbench/common/theme.ts',
    'src/vs/editor/common/core/editorColorRegistry.ts',
    'src/vs/editor/contrib/folding/browser/foldingDecorations.ts',
];
const symbols = new Map(), registered = new Map(), aliases = new Map();
for (const filename of paths) {
    const source = ts.createSourceFile(filename, fs.readFileSync(path.join(root, filename), 'utf8'), ts.ScriptTarget.Latest, true);
    function walk(node) {
        if (ts.isVariableDeclaration(node) && node.initializer)
            aliases.set(node.name.getText(source), { node: node.initializer, source });
        if (ts.isVariableDeclaration(node) && node.initializer && ts.isCallExpression(node.initializer) && node.initializer.expression.getText(source) === 'registerColor') {
            const [id, def] = node.initializer.arguments;
            if (ts.isStringLiteral(id)) {
                symbols.set(node.name.getText(source), id.text);
                registered.set(id.text, { node: def, source, filename });
            }
        }
        ts.forEachChild(node, walk);
    }
    walk(source);
}
function number(n) { if (ts.isNumericLiteral(n))
    return Number(n.text); if (ts.isPrefixUnaryExpression(n) && n.operator === ts.SyntaxKind.MinusToken)
    return -number(n.operand); throw Error('not numeric: ' + n.getText()); }
function hex(r, g, b, a = 1) { return '#' + [r, g, b, ...(a === 1 ? [] : [Math.round(a * 255)])].map(v => v.toString(16).padStart(2, '0')).join('').toUpperCase(); }
function expression(n, source) {
    if (n.kind === ts.SyntaxKind.NullKeyword)
        return null;
    if (ts.isStringLiteral(n))
        return n.text.startsWith('#') ? { hex: n.text.toUpperCase() } : { ref: n.text };
    if (ts.isIdentifier(n)) {
        const ref = symbols.get(n.text);
        if (ref)
            return { ref };
        const alias = aliases.get(n.text);
        if (alias)
            return expression(alias.node, alias.source);
        throw Error('unknown symbol ' + n.text);
    }
    if (ts.isObjectLiteralExpression(n)) {
        const result = {};
        for (const p of n.properties) {
            result[p.name.getText(source)] = expression(p.initializer, source);
        }
        return { variants: result };
    }
    if (ts.isPropertyAccessExpression(n)) {
        const values = { 'Color.white': '#FFFFFF', 'Color.black': '#000000', 'Color.transparent': '#00000000' };
        const value = values[n.getText(source)];
        if (value)
            return { hex: value };
    }
    if (ts.isNewExpression(n) && n.expression.getText(source) === 'Color') {
        const inner = n.arguments[0];
        if (ts.isNewExpression(inner) && inner.expression.getText(source) === 'RGBA')
            return { hex: hex(...inner.arguments.map(number)) };
    }
    if (ts.isCallExpression(n)) {
        const name = n.expression.getText(source), args = [...n.arguments];
        if (name === 'Color.fromHex')
            return expression(args[0], source);
        if (['transparent', 'oneOf'].includes(name))
            return { op: name, args: args.map(a => ts.isNumericLiteral(a) ? number(a) : expression(a, source)) };
        if (ts.isPropertyAccessExpression(n.expression) && n.expression.name.text === 'transparent')
            return { op: 'transparent', args: [expression(n.expression.expression, source), number(args[0])] };
    }
    throw Error('unsupported default: ' + n.getText(source));
}
const palette = fs.readFileSync(path.join(__dirname, '../colors.go'), 'utf8');
const requested = new Set([...palette.matchAll(/(?:get|t\.Color|t\.colorOr)\("([A-Za-z][A-Za-z0-9.]+)"/g)].map(m => m[1]));
const entries = {}, sources = {};
const git = JSON.parse(fs.readFileSync(gitPackage, 'utf8')).contributes.colors;
for (const entry of git) {
    if (entry.id.startsWith('gitDecoration.')) {
        const d = entry.defaults;
        entries[entry.id] = { variants: { dark: { hex: d.dark }, light: { hex: d.light }, hcDark: { hex: d.highContrast }, hcLight: { hex: d.highContrastLight } } };
        sources[entry.id] = 'extensions/git/package.json';
    }
}
function include(id) {
    if (id in entries)
        return;
    const entry = registered.get(id);
    if (!entry)
        throw Error('unregistered requested color ' + id);
    entries[id] = expression(entry.node, entry.source);
    sources[id] = entry.filename;
    function visit(value) { if (!value || typeof value !== 'object')
        return; if (value.ref)
        include(value.ref); for (const v of Object.values(value))
        visit(v); }
    visit(entries[id]);
}
for (const id of requested)
    include(id);
const sort = o => Object.fromEntries(Object.entries(o).sort(([a], [b]) => a.localeCompare(b)));
fs.writeFileSync(path.join(__dirname, '../defaults.json'), JSON.stringify({ revision: fs.existsSync(path.join(root, 'revision.txt')) ? fs.readFileSync(path.join(root, 'revision.txt'), 'utf8').trim() : require('child_process').execFileSync('git', ['-C', root, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(), sources: sort(sources), colors: sort(entries) }, null, 2) + '\n');
console.log(Object.keys(entries).length + ' registered color defaults extracted');
