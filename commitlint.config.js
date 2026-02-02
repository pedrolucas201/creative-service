module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'type-enum': [
      2,
      'always',
      [
        'feat',     // Nova feature
        'fix',      // Bug fix
        'docs',     // Documentação
        'style',    // Formatação (não afeta código)
        'refactor', // Refatoração
        'perf',     // Performance
        'test',     // Testes
        'chore',    // Manutenção/config
        'ci',       // CI/CD
        'build',    // Build system
        'revert',   // Revert commit
      ],
    ],
    'subject-case': [0], // Permite qualquer case no subject
    'subject-max-length': [2, 'always', 100],
    'body-max-line-length': [0], // Desabilita limite de linha no body
  },
};
