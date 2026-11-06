const js = require('@eslint/js');
const globals = require('globals');
const tseslint = require('typescript-eslint');

const commentSpacingRule = {
  meta: {
    type: 'layout',
    docs: { description: 'Require one blank line after standalone explanatory comment blocks.' },
    fixable: 'whitespace',
    schema: [],
    messages: { spacing: 'Leave exactly one blank line after this standalone comment block.' },
  },
  create(context) {
    const source = context.sourceCode;

    return {
      'Program:exit'() {
        const comments = source.getAllComments();

        for (let index = 0; index < comments.length; index++) {
          const comment = comments[index];
          let start = comment.range[0];
          let end = comment.range[1];
          const node = source.getNodeByRangeIndex(start);

          // Treat an empty JSX comment container as a single standalone unit.
          if (node?.type === 'JSXEmptyExpression') {
            start = node.parent.range[0];
            end = node.parent.range[1];
          }

          const lineStart = source.text.lastIndexOf('\n', start - 1) + 1;
          const lineEnd = source.text.indexOf('\n', end);
          if (lineEnd === -1 || source.text.slice(lineStart, start).trim()) continue;
          if (source.text.slice(end, lineEnd).trim()) continue;

          // Keep directive comments attached to the code whose tooling behavior they control.
          if (
            /^\s*(?:eslint\b|eslint-|prettier-ignore\b|@ts-|istanbul\b|c8\b)/.test(comment.value)
          ) {
            continue;
          }

          let nextLineStart = lineEnd + 1;
          while (nextLineStart < source.text.length) {
            const nextLineEnd = source.text.indexOf('\n', nextLineStart);
            const boundary = nextLineEnd === -1 ? source.text.length : nextLineEnd;
            if (source.text.slice(nextLineStart, boundary).trim()) break;
            nextLineStart = boundary + 1;
          }
          if (nextLineStart >= source.text.length) continue;

          const next = source.text.slice(nextLineStart).trimStart();
          const followingComment = comments[index + 1];
          if (
            followingComment &&
            /^\r?\n[ \t]*(?:\{[ \t]*)?$/.test(source.text.slice(end, followingComment.range[0]))
          ) {
            continue;
          }

          // Closing delimiters do not introduce a related code section.
          if (/^[}\])]/.test(next)) continue;
          if (source.text.slice(lineEnd + 1, nextLineStart) === '\n') continue;

          context.report({
            loc: comment.loc,
            messageId: 'spacing',
            fix: fixer => fixer.replaceTextRange([lineEnd + 1, nextLineStart], '\n'),
          });
        }
      },
    };
  },
};

module.exports = tseslint.config(
  {
    ignores: [
      'node_modules/**',
      '.cache/**',
      '.expo/**',
      '.npm-cache/**',
      'dist/**',
      'scripts/check-*.cjs',
      'scripts/verify-served-recovery.cjs',
      'scripts/historical-paths.cjs',
    ],
  },
  {
    files: ['scripts/*.cjs', 'eslint.config.cjs'],
    ...js.configs.recommended,
    languageOptions: { globals: globals.node },
  },
  {
    files: ['**/*.ts', '**/*.tsx'],
    plugins: { local: { rules: { 'comment-spacing': commentSpacingRule } } },
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    languageOptions: {
      globals: { ...globals.node, ...globals.browser, __DEV__: 'readonly' },
    },
    rules: {
      'local/comment-spacing': 'error',
      // Metro needs static asset requires and guarded development-only screen requires.
      '@typescript-eslint/no-require-imports': 'off',
      // Existing callback signatures include intentionally unused props and native coordinates.
      '@typescript-eslint/no-unused-vars': ['warn', { args: 'none', varsIgnorePattern: '^_' }],
    },
  },
);
