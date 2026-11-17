// Run: NODE_PATH=/path/to/node_modules bun internal/theme/testdata/generate.cjs
// Reference: microsoft/vscode-textmate 9.3.2 (MIT). No runtime dependency.
const fs = require('fs');
const { Registry } = require('vscode-textmate');
const tokenColors = [
    { scope: 'variable', settings: { foreground: '#123456', fontStyle: 'italic' } },
    { scope: 'variable.other', settings: { fontStyle: 'bold' } },
    { scope: 'source.go variable', settings: { foreground: '#abcdef' } },
    { scope: 'source.go variable.other', settings: { fontStyle: '' } },
    { scope: 'source.go meta.block variable.other', settings: { foreground: '#334455', fontStyle: 'underline' } },
    { scope: 'source.go > variable.other', settings: { foreground: '#556677' } },
    { scope: 'meta.block variable.other', settings: { foreground: '#778899' } },
    { scope: 'meta.block variable.other', settings: { fontStyle: 'strikethrough' } },
    { scope: 'variable.other.member', settings: { foreground: '#987654' } },
    { scope: 'comment, string', settings: { foreground: '#369', fontStyle: 'italic bold' } },
    { scope: ['keyword', 'storage'], settings: { foreground: '#dddddd' } },
    { scope: 'keyword.control', settings: { fontStyle: 'italic' } },
    { scope: 'source.go keyword.control', settings: { foreground: '#fedcba' } },
    { scope: 'source.go > meta.block > keyword.control', settings: { fontStyle: '' } },
    { scope: 'source.go meta.block keyword', settings: { foreground: '#555555' } },
    { scope: 'source.go meta.block keyword.control', settings: { foreground: '#666666' } },
    { scope: 'keyword.control', settings: { foreground: '#777777' } },
    // Global unscoped token rules are filtered out by VS Code's ColorThemeData.
    { settings: { foreground: '#ff0000', fontStyle: 'bold' } },
];
const registry = new Registry({ loadGrammar: async () => null });
registry.setTheme({ settings: [{ settings: { foreground: '#BBBBBB', background: '#1E1E1E' } }, ...tokenColors.filter(r => r.scope)] });
const theme = registry._syncRegistry._theme;
const colorMap = registry.getColorMap();
const contexts = [[], ['source.go'], ['source.go', 'meta.block'], ['source.go', 'meta.block', 'meta.inner'], ['source.js'], ['source.js', 'meta.block'], ['source.go', 'meta.block', 'source.js']];
const leaves = ['variable', 'variable.other', 'variable.other.member.go', 'keyword', 'keyword.control.go', 'comment.line', 'string.quoted', 'storage.type', 'unknown'];
const cases = [];
for (const context of contexts)
    for (const leaf of leaves) {
        const scopes = [...context, leaf];
        let parent = null, color = '#BBBBBB', font = 0;
        for (const scopeName of scopes) {
            parent = { scopeName, parent };
            const style = theme.match(parent);
            if (!style)
                continue;
            if (style.foregroundId)
                color = colorMap[style.foregroundId];
            if (style.fontStyle !== -1)
                font = style.fontStyle;
        }
        // Upstream enum uses italic=1,bold=2; vl uses bold=1,italic=2.
        font = (font & 1 ? 2 : 0) | (font & 2 ? 1 : 0) | (font & 12);
        if (color.length === 4)
            color = "#" + [...color.slice(1)].map(c => c + c).join("");
        cases.push({ scopes, color, font });
    }
fs.writeFileSync(__dirname + '/vscode-textmate.json', JSON.stringify({ reference: 'vscode-textmate@9.3.2', tokenColors, cases }, null, 2) + '\n');
