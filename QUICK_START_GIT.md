# 🚀 Quick Start - Setup GitHub Profissional

Este projeto está configurado com Husky + Commitlint para commits convencionais e estrutura de branches profissional.

## ⚡ Setup Automatizado (Recomendado)

### Passo 1: Configurar Husky
```powershell
.\setup-github.ps1
```

### Passo 2: Criar Branches Temáticas
```powershell
.\run-git-setup-branches.ps1
```

### Passo 3: Criar Pull Requests no GitHub
- Acesse seu repositório no GitHub
- Crie PRs de cada feature branch → develop
- Revise e faça merge

---

## 📖 Documentação Completa

Para processo manual detalhado e explicações, consulte:
- **[SETUP_GITHUB.md](SETUP_GITHUB.md)** - Guia completo passo a passo

---

## 📝 Padrão de Commits

Use **Conventional Commits**:

```bash
feat(api): adicionar endpoint de campanhas
fix(worker): corrigir race condition
docs(readme): atualizar documentação
refactor(meta): extrair lógica de retry
chore(deps): atualizar dependências
```

### Tipos permitidos:
- `feat` - Nova funcionalidade
- `fix` - Correção de bug
- `docs` - Documentação
- `refactor` - Refatoração
- `test` - Testes
- `chore` - Manutenção
- `perf` - Performance
- `ci` - CI/CD

---

## 🌿 Estrutura de Branches

```
main (produção)
  ↑
develop (staging)
  ↑
feature/nome-da-feature (features)
fix/nome-do-bug (correções)
docs/nome-da-doc (documentação)
```

---

## 🆘 Troubleshooting

### Commitlint não valida
```powershell
# Testar manualmente
echo "feat: teste" | npx commitlint
```

### Husky não funciona
```powershell
# Reinstalar
rm -rf node_modules .husky
npm install
npx husky install
```

### Erro de permissão (Windows)
```powershell
Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned
```

---

## 📚 Recursos

- [Conventional Commits](https://www.conventionalcommits.org/)
- [Husky](https://typicode.github.io/husky/)
- [Commitlint](https://commitlint.js.org/)

---

**Dica**: Execute `git log --oneline --graph --all` para visualizar a árvore de branches! 🌳
