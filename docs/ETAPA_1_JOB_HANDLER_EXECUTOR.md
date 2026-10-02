# Etapa 1 — Job, Handler e Executor

Este documento resume a primeira etapa do projeto **GoFlow**, cujo objetivo é construir gradualmente um sistema de execução de jobs em Go enquanto revisamos conceitos da linguagem, backend, concorrência e sistemas distribuídos.

Nesta etapa, o foco foi criar a base do domínio **sem concorrência, filas, banco de dados ou HTTP**.

---

## Objetivo da etapa

Queremos representar uma unidade de trabalho e executá-la através do componente responsável por aquele tipo de tarefa.

A arquitetura inicial é:

```text
Job
 │
 ▼
Executor
 │
 ▼
Handler
 │
 ▼
Handle()
```

As responsabilidades foram separadas da seguinte forma:

```text
Job      → representa o trabalho
Handler  → sabe como executar um tipo de trabalho
Executor → encontra o Handler correto e dispara a execução
```

---

# 1. Job

O `Job` representa uma unidade de trabalho que precisa ser executada.

Exemplo:

```go
type Job struct {
	ID        string
	Type      string
	Payload   []byte
	Status    Status
	CreatedAt time.Time
}
```

Podemos ter jobs como:

```text
send_email
process_payment
generate_report
```

O campo `Type` informa ao sistema qual tipo de trabalho precisa ser executado.

Por exemplo:

```go
job := Job{
	ID:     "job-123",
	Type:   "send_email",
	Status: StatusPending,
}
```

O `Payload` contém as informações necessárias para executar o trabalho.

Inicialmente usamos:

```go
Payload []byte
```

Isso deixa o core do sistema independente do formato utilizado no payload.

Ele poderia conter JSON, por exemplo:

```json
{
  "email": "user@example.com",
  "subject": "Welcome!"
}
```

---

# 2. Handler

O `Handler` define o contrato para qualquer componente que saiba executar um job.

```go
type Handler interface {
	Handle(ctx context.Context, job Job) error
}
```

Um handler pode ser:

```text
EmailHandler
PaymentHandler
ReportHandler
```

Exemplo:

```go
type EmailHandler struct{}

func (h EmailHandler) Handle(ctx context.Context, job Job) error {
	fmt.Printf("sending email for job %s\n", job.ID)
	return nil
}
```

## Interfaces em Go

Em Go não existe a necessidade de declarar explicitamente:

```text
EmailHandler implements Handler
```

A implementação de interfaces é **implícita**.

Se um tipo possui todos os métodos exigidos pela interface, ele automaticamente satisfaz aquela interface.

Se temos:

```go
type Handler interface {
	Handle(ctx context.Context, job Job) error
}
```

e:

```go
func (h EmailHandler) Handle(ctx context.Context, job Job) error
```

então:

```text
EmailHandler satisfaz Handler
```

Isso reduz o acoplamento entre implementações e interfaces.

---

# 3. Executor

O `Executor` é responsável por conectar um `Job` ao `Handler` apropriado.

Ele **não sabe executar pagamentos, enviar emails ou gerar relatórios**.

Sua responsabilidade é apenas:

```text
1. receber um Job
2. verificar o Job.Type
3. localizar o Handler correspondente
4. chamar Handler.Handle()
```

A estrutura é:

```go
type Executor struct {
	handlers map[string]Handler
}
```

Esse mapa funciona conceitualmente assim:

```text
"send_email"      → EmailHandler
"process_payment" → PaymentHandler
"generate_report" → ReportHandler
```

---

# 4. Criando o Executor

O construtor inicializa o mapa de handlers:

```go
func NewExecutor() *Executor {
	return &Executor{
		handlers: make(map[string]Handler),
	}
}
```

Retornamos:

```go
*Executor
```

porque `Executor` é uma `struct` e queremos trabalhar com a mesma instância ao registrar e executar handlers.

---

# 5. Registrando handlers

O método `Register` associa um tipo de job a um handler:

```go
func (e *Executor) Register(jobType string, handler Handler) {
	e.handlers[jobType] = handler
}
```

Exemplo:

```go
executor.Register(
	"send_email",
	EmailHandler{},
)
```

Depois disso:

```text
send_email → EmailHandler{}
```

está registrado no executor.

---

# 6. Executando um Job

O método `Execute` procura o handler correspondente ao `Job.Type`.

```go
func (e *Executor) Execute(ctx context.Context, job Job) error {
	handler, ok := e.handlers[job.Type]

	if !ok {
		return fmt.Errorf(
			"handler not found for job type %q",
			job.Type,
		)
	}

	return handler.Handle(ctx, job)
}
```

O fluxo é:

```text
Job
 │
 │ Type = "send_email"
 ▼
Executor
 │
 │ handlers["send_email"]
 ▼
EmailHandler
 │
 ▼
Handle(ctx, job)
```

---

# 7. Por que não usar if/switch para cada tipo?

Poderíamos escrever algo assim:

```go
func (e *Executor) Execute(job Job) error {
	if job.Type == "send_email" {
		// enviar email
	}

	if job.Type == "payment" {
		// processar pagamento
	}

	return nil
}
```

Mas isso faria o `Executor` conhecer todas as regras de negócio.

Com o crescimento do sistema teríamos:

```text
send_email
process_payment
generate_report
refund
notification
invoice
webhook
...
```

e o método `Execute` cresceria continuamente.

Com handlers registrados:

```go
map[string]Handler
```

o Executor permanece simples.

Ele apenas despacha o trabalho.

---

# 8. Ponteiros e interfaces

Um ponto importante revisado nesta etapa foi a diferença entre ponteiros para structs e ponteiros para interfaces.

## Struct

É comum trabalhar com:

```go
*Executor
```

ou:

```go
*Job
```

porque são structs.

## Interface

Normalmente não usamos:

```go
*Handler
```

Usamos:

```go
Handler
```

Da mesma forma, `context.Context` também é uma interface.

O idiomático é:

```go
func Execute(ctx context.Context)
```

e não:

```go
func Execute(ctx *context.Context)
```

Regra prática:

```text
struct                     interface
------                     ---------

Job                        Handler
Executor                   context.Context

*Job       ✅ possível      *Handler         ⚠️ quase nunca
*Executor  ✅ comum         *context.Context ⚠️ quase nunca
```

---

# 9. Encapsulamento

O campo do Executor foi definido como:

```go
handlers map[string]Handler
```

e não:

```go
Handlers map[string]Handler
```

Como começa com letra minúscula, ele só pode ser acessado diretamente dentro do package `job`.

Isso impede código externo de fazer:

```go
executor.handlers["qualquer-coisa"] = handler
```

O caminho oficial passa a ser:

```go
executor.Register(...)
```

Isso melhora o encapsulamento e permite adicionar validações futuramente.

---

# 10. Tratamento de erros

Em vez de retornar apenas:

```go
errors.New("handler not found")
```

preferimos adicionar contexto:

```go
fmt.Errorf(
	"handler not found for job type %q",
	job.Type,
)
```

Isso produz um erro como:

```text
handler not found for job type "send_sms"
```

Em produção, erros com contexto facilitam debugging, logs e observabilidade.

---

# Implementação final do Executor

```go
package job

import (
	"context"
	"fmt"
)

type Executor struct {
	handlers map[string]Handler
}

func NewExecutor() *Executor {
	return &Executor{
		handlers: make(map[string]Handler),
	}
}

func (e *Executor) Register(jobType string, handler Handler) {
	e.handlers[jobType] = handler
}

func (e *Executor) Execute(ctx context.Context, job Job) error {
	handler, ok := e.handlers[job.Type]

	if !ok {
		return fmt.Errorf(
			"handler not found for job type %q",
			job.Type,
		)
	}

	return handler.Handle(ctx, job)
}
```

---

# Modelo mental

A principal ideia desta etapa é:

> **O Job descreve o trabalho. O Handler sabe executá-lo. O Executor conecta os dois.**

Ou:

```text
        informação
            │
            ▼
           Job
            │
            ▼
        Executor
            │
     encontra Handler
            │
            ▼
         Handler
            │
          Handle()
            │
            ▼
        trabalho real
```

Essa separação será importante quando adicionarmos concorrência.

No futuro, o fluxo será algo como:

```text
Queue
  │
  ▼
Worker
  │
  ▼
Executor
  │
  ▼
Handler
```

O `Worker` não precisará conhecer `EmailHandler`, `PaymentHandler` ou qualquer regra de negócio.

Ele simplesmente receberá um `Job` e chamará:

```go
executor.Execute(ctx, job)
```

---

# Conceitos de Go revisados nesta etapa

- `struct`
- tipos customizados
- constantes
- interfaces
- implementação implícita de interfaces
- maps
- métodos
- pointer receivers
- constructors idiomáticos (`New...`)
- `context.Context`
- tratamento explícito de erros
- encapsulamento através de identificadores não exportados
- separação de responsabilidades
- dependency inversion através de interfaces

---

# Próxima etapa

Antes de introduzir concorrência, o próximo passo é:

```text
1. criar um EmailHandler real
2. criar o main.go
3. executar um Job através do Executor
4. escrever testes para Executor
```

Depois disso evoluiremos de:

```text
1 Job
  │
  ▼
execução síncrona
```

para:

```text
             Channel
                │
       ┌────────┼────────┐
       ▼        ▼        ▼
    Worker 1 Worker 2 Worker 3
       │        │        │
       └────────┼────────┘
                ▼
             Executor
```

Nesse ponto começaremos a trabalhar com:

- goroutines
- channels
- worker pools
- `sync.WaitGroup`
- cancellation
- graceful shutdown
- goroutine leaks
