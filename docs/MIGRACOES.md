# Migrations com golang-migrate

Pré-requisitos: Go para os comandos da aplicação e Docker com Compose para o
PostgreSQL e as migrations. O CLI do golang-migrate roda em um container;
não é necessário instalar o CLI localmente ou adicionar uma dependência ao go.mod.

```sh
make help
make run
make test
make build
```

## Criar e aplicar

```sh
make migrate-create NAME=create_jobs
```

Esse comando cria `migrations/000001_create_jobs.up.sql` e
`migrations/000001_create_jobs.down.sql` na primeira execução. Preencha o arquivo
`up.sql` com a alteração do banco e o `down.sql` com a operação que a reverte.
As migrations seguintes recebem números sequenciais. Não altere migrations
que já foram aplicadas; crie uma nova para a próxima alteração.

```sh
make migrate-up
make migrate-version
make migrate-down
```

Os comandos de banco iniciam o PostgreSQL e esperam seu healthcheck antes de
executar o migrate. `migrate-up` aplica todas as migrations pendentes;
`migrate-down` reverte apenas a última e pode remover dados, conforme o SQL escrito.
A pasta começa vazia porque o esquema do banco ainda precisa ser definido.

O endereço padrão é
`postgres://goflow:goflow@postgres:5432/goflow?sslmode=disable`.
O hostname `postgres` funciona dentro da rede do Compose. Para conectar uma
ferramenta no computador, use `localhost:5432`, usuário, senha e banco `goflow`.
Essas credenciais são para desenvolvimento local.

Para outro banco, informe um endereço acessível a partir do container:

```sh
make migrate-up DATABASE_URL='postgres://usuario:senha@host:5432/banco?sslmode=require'
```

Se uma migration falhar e a versão ficar marcada como `dirty`, examine o erro e
o estado do banco antes de ajustar a versão manualmente. O comando `force` do
migrate altera o registro de versão sem executar SQL.

Referência: [documentação oficial do golang-migrate](https://github.com/golang-migrate/migrate/blob/master/GETTING_STARTED.md).
