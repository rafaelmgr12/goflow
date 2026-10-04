# Etapa 2 — Concorrência, Worker Pool e Graceful Shutdown

Este documento resume a segunda etapa do projeto **GoFlow**.

Nesta fase, evoluímos o sistema de execução síncrona da Etapa 1 para um modelo concorrente com:

- goroutines
- channels
- worker pool
- `sync.WaitGroup`
- `context.Context`
- cancellation
- graceful shutdown
- sinais do sistema operacional
- testes concorrentes
- race detector

O objetivo foi entender como estruturar um serviço concorrente em Go sem perder controle sobre lifecycle, shutdown e sincronização.

---

## 1. Objetivo da etapa

Na Etapa 1 tínhamos:

```text
Job
 ↓
Executor
 ↓
Handler
```

Um job era executado por vez.

Nesta etapa evoluímos para:

```text
                jobs channel
                     │
        ┌────────────┼────────────┐
        ▼            ▼            ▼
     Worker 1     Worker 2     Worker 3
        │            │            │
        └────────────┼────────────┘
                     ▼
                  Executor
                     │
                     ▼
                  Handler
```

Agora vários workers podem consumir jobs concorrentemente.

---

## 2. Goroutines

Uma função normal:

```go
worker()
```

bloqueia até terminar.

Com:

```go
go worker()
```

ela passa a rodar concorrentemente.

Goroutines são leves, mas isso não significa que devemos criar uma por job sem controle.

Este padrão pode ser perigoso:

```go
for _, job := range jobs {
    go executor.Execute(ctx, job)
}
```

Se entrarem milhões de jobs, podemos criar milhões de goroutines e saturar memória, banco, APIs externas e o scheduler do runtime.

Por isso usamos um **worker pool**.

---

## 3. Worker Pool

O worker pool limita quantos jobs podem executar simultaneamente.

```text
1000 jobs
    │
    ▼
jobs channel
    │
┌────────┬────────┬────────┐
│Worker 1│Worker 2│Worker 3│
└────────┴────────┴────────┘
```

Mesmo com 1000 jobs, apenas 3 são processados simultaneamente quando existem 3 workers.

Isso cria um limite explícito de concorrência e uma forma de backpressure.

---

## 4. Worker

O worker fica esperando jobs e os delega ao `Executor`.

Versão inicial:

```go
func Run(
    ctx context.Context,
    id int,
    jobs <-chan job.Job,
    executor *job.Executor,
) {
    for j := range jobs {
        log.Printf("worker %d processing job %s", id, j.ID)

        if err := executor.Execute(ctx, j); err != nil {
            log.Printf(
                "worker %d failed job %s: %v",
                id,
                j.ID,
                err,
            )
        }
    }
}
```

Fluxo:

```text
esperar job
    ↓
receber job
    ↓
chamar Executor
    ↓
voltar a esperar
```

---

## 5. Channels

Criamos uma fila em memória:

```go
jobs := make(chan job.Job)
```

Para enviar:

```go
jobs <- j
```

Para receber:

```go
j := <-jobs
```

Fluxo:

```text
Producer
   │
   ▼
jobs channel
   │
   ▼
Worker
```

---

## 6. Directional Channels

Na assinatura do worker usamos:

```go
jobs <-chan job.Job
```

Isso significa que o parâmetro só pode **receber** valores do channel.

```go
chan job.Job
```

envia e recebe.

```go
<-chan job.Job
```

somente recebe.

```go
chan<- job.Job
```

somente envia.

Isso melhora a API porque expressa a intenção diretamente na assinatura.

---

## 7. Fechando channels

Um worker pode consumir assim:

```go
for j := range jobs {
    // processar
}
```

Esse loop termina quando o channel é fechado.

O produtor faz:

```go
close(jobs)
```

Fluxo:

```text
producer terminou
      ↓
close(jobs)
      ↓
worker termina o range
      ↓
worker encerra
```

Regra prática:

> Quem controla a produção normalmente é quem fecha o channel.

Enviar para um channel fechado gera:

```text
panic: send on closed channel
```

---

## 8. Recebendo com `value, ok`

Também usamos:

```go
j, ok := <-jobs
```

Quando existe um valor:

```text
j  = valor recebido
ok = true
```

Quando o channel foi fechado e está vazio:

```text
j  = zero value
ok = false
```

Por isso:

```go
if !ok {
    return
}
```

Isso evita processar um `Job{}` vazio.

---

## 9. WaitGroup

Goroutines não mantêm o processo vivo. Se `main` terminar, o processo inteiro termina.

Por isso usamos:

```go
var wg sync.WaitGroup
```

Antes de iniciar cada worker:

```go
wg.Add(1)
```

Dentro da goroutine:

```go
defer wg.Done()
```

No final:

```go
wg.Wait()
```

Exemplo:

```go
for i := 1; i <= 3; i++ {
    wg.Add(1)

    go func(workerID int) {
        defer wg.Done()

        worker.Run(
            ctx,
            workerID,
            jobs,
            executor,
        )
    }(i)
}
```

Modelo mental:

```text
counter = 0

Add(1) → 1
Add(1) → 2
Add(1) → 3

worker 1 termina → Done() → 2
worker 2 termina → Done() → 1
worker 3 termina → Done() → 0

Wait() desbloqueia
```

---

## 10. Concorrência não garante ordem

Com vários workers, a ordem não é garantida.

```text
job-1 → worker 2
job-2 → worker 1
job-3 → worker 3
job-4 → worker 2
```

Em outra execução, pode ser diferente.

Portanto:

```text
ordem de envio != ordem de processamento
ordem de processamento != ordem de conclusão
```

Não devemos depender da ordem entre goroutines sem sincronização explícita.

---

## 11. `context.Context`

Evoluímos o worker para respeitar cancelamento:

```go
for {
    select {
    case <-ctx.Done():
        return

    case j, ok := <-jobs:
        if !ok {
            return
        }

        if err := executor.Execute(ctx, j); err != nil {
            log.Printf("failed: %v", err)
        }
    }
}
```

Agora o worker espera por duas coisas:

```text
              Worker
                 │
        ┌────────┴────────┐
        ▼                 ▼
   chegou Job?       ctx cancelou?
        │                 │
        ▼                 ▼
   processa Job         termina
```

---

## 12. `ctx.Done()`

`ctx.Done()` retorna um channel.

Enquanto o contexto está ativo, esse channel fica bloqueado.

Quando ocorre:

```go
cancel()
```

o channel é fechado, permitindo que o `select` execute:

```go
case <-ctx.Done():
```

---

## 13. Context não mata goroutines

`context` não interrompe código à força.

Ele apenas fornece um sinal cooperativo de cancelamento.

Exemplo que não responde ao contexto:

```go
time.Sleep(30 * time.Second)
```

Exemplo cooperativo:

```go
select {
case <-time.After(30 * time.Second):
    // terminou

case <-ctx.Done():
    return ctx.Err()
}
```

O mesmo conceito aparece em APIs reais:

```go
db.QueryContext(ctx, query)
```

```go
http.NewRequestWithContext(ctx, method, url, body)
```

---

## 14. Graceful Shutdown

Ligamos o contexto aos sinais do sistema operacional:

```go
ctx, stop := signal.NotifyContext(
    context.Background(),
    os.Interrupt,
    syscall.SIGTERM,
)
defer stop()
```

Agora `Ctrl+C` ou `SIGTERM` sinalizam shutdown.

Isso é importante em ambientes como Docker, Kubernetes e systemd.

---

## 15. Shutdown do worker vs cancelamento do job

Uma decisão importante de arquitetura:

```text
shutdown do worker
```

não é necessariamente igual a:

```text
cancelar o job em execução
```

Em muitos casos queremos:

```text
SIGTERM
   ↓
worker não pega novos jobs
   ↓
job atual termina
   ↓
worker encerra
```

Isso é especialmente importante para operações como pagamentos, refunds e webhooks.

---

## 16. `context.WithoutCancel`

Para separar o lifecycle do worker do lifecycle do job, usamos:

```go
jobCtx := context.WithoutCancel(ctx)
```

Assim:

```text
ctx
 └── controla shutdown do worker

jobCtx
 └── execução atual não é cancelada pelo shutdown
```

Exemplo:

```go
jobCtx := context.WithoutCancel(ctx)

if err := executor.Execute(jobCtx, j); err != nil {
    log.Printf("failed: %v", err)
}
```

---

## 17. Timeout do job

Em produção não queremos esperar eternamente por um job travado.

Podemos combinar:

```go
baseCtx := context.WithoutCancel(ctx)

jobCtx, cancel := context.WithTimeout(
    baseCtx,
    30*time.Second,
)
defer cancel()
```

Assim:

```text
shutdown
   ↓
não cancela imediatamente o job atual

job ultrapassa 30s
   ↓
timeout
   ↓
jobCtx cancelado
```

---

## 18. Lifecycle final

O lifecycle desejado ficou:

```text
SIGTERM / Ctrl+C
       │
       ▼
context cancelado
       │
       ▼
producer para de gerar jobs
       │
       ▼
workers param de buscar novos jobs
       │
       ▼
jobs atuais terminam
       │
       ▼
workers encerram
       │
       ▼
WaitGroup chega a zero
       │
       ▼
processo termina
```

---

## 19. Testando código concorrente

Criamos um fake handler usando channel:

```go
type fakeHandler struct {
    called chan job.Job
}

func (f *fakeHandler) Handle(
    ctx context.Context,
    j job.Job,
) error {
    f.called <- j
    return nil
}
```

Em vez de compartilhar um `bool` entre goroutines, usamos um channel para comunicar que o handler foi executado.

---

## 20. Teste de processamento

O teste valida:

```text
job enviado
    ↓
worker recebe
    ↓
Executor executa
    ↓
fakeHandler recebe
```

Exemplo:

```go
func TestWorkerProcessesJob(t *testing.T) {
    ctx := context.Background()

    handler := &fakeHandler{
        called: make(chan job.Job, 1),
    }

    executor := job.NewExecutor()
    executor.Register("send_email", handler)

    jobs := make(chan job.Job)
    done := make(chan struct{})

    go func() {
        Run(ctx, 1, jobs, executor)
        close(done)
    }()

    expectedJob := job.Job{
        ID:     "job-1",
        Type:   "send_email",
        Status: job.StatusPending,
    }

    jobs <- expectedJob

    select {
    case receivedJob := <-handler.called:
        if receivedJob.ID != expectedJob.ID {
            t.Fatalf(
                "expected job %s, got %s",
                expectedJob.ID,
                receivedJob.ID,
            )
        }

    case <-time.After(time.Second):
        t.Fatal("worker did not process job")
    }

    close(jobs)

    select {
    case <-done:
    case <-time.After(time.Second):
        t.Fatal("worker did not shut down")
    }
}
```

---

## 21. Por que usar timeout nos testes?

Evite:

```go
receivedJob := <-handler.called
```

Se houver um bug, o teste pode bloquear para sempre.

Prefira:

```go
select {
case receivedJob := <-handler.called:
    // sucesso

case <-time.After(time.Second):
    t.Fatal("timeout")
}
```

---

## 22. Testando cancellation

Também testamos se o worker termina ao cancelar o contexto:

```go
func TestWorkerStopsWhenContextIsCancelled(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())

    executor := job.NewExecutor()
    jobs := make(chan job.Job)
    done := make(chan struct{})

    go func() {
        Run(ctx, 1, jobs, executor)
        close(done)
    }()

    cancel()

    select {
    case <-done:
    case <-time.After(time.Second):
        t.Fatal("worker did not stop after context cancellation")
    }
}
```

Aqui o channel `jobs` continua aberto. O worker termina porque `ctx.Done()` foi disparado.

Temos dois mecanismos diferentes:

```text
close(jobs)
    ↓
fim normal

cancel()
    ↓
shutdown/cancelamento
```

---

## 23. Data Race

Um data race acontece quando duas goroutines acessam a mesma região de memória concorrentemente e pelo menos uma delas escreve, sem sincronização apropriada.

Exemplo perigoso:

```go
type fakeHandler struct {
    called bool
}
```

Uma goroutine escreve:

```go
f.called = true
```

Enquanto outra lê:

```go
if f.called {
    // ...
}
```

Sem `mutex`, `atomic`, channel ou outra forma de sincronização, isso pode ser um data race.

---

## 24. Race Detector

Go possui uma ferramenta própria para detectar races:

```bash
go test -race ./...
```

Também devemos rodar normalmente:

```bash
go test ./...
```

Objetivo:

```text
go test ./...       ✅
go test -race ./... ✅
```

Se houver problema, o Go pode mostrar:

```text
WARNING: DATA RACE
```

---

## 25. Conceitos revisados

Nesta etapa estudamos:

- goroutines
- channels
- directional channels
- worker pool
- backpressure
- `close(channel)`
- `value, ok := <-channel`
- `sync.WaitGroup`
- `select`
- `context.Context`
- `context.WithCancel`
- `context.WithoutCancel`
- cancellation cooperativo
- timeouts
- `signal.NotifyContext`
- `SIGTERM`
- `Ctrl+C`
- graceful shutdown
- lifecycle de goroutines
- testes concorrentes
- sincronização via channels
- data races
- race detector

---

## 26. Modelo mental final

```text
                 Producer
                    │
                    ▼
             ┌─────────────┐
             │ jobs channel│
             └──────┬──────┘
                    │
       ┌────────────┼────────────┐
       ▼            ▼            ▼
    Worker 1     Worker 2     Worker 3
       │            │            │
       └────────────┼────────────┘
                    ▼
                 Executor
                    │
                    ▼
                 Handler
```

Lifecycle:

```text
SIGTERM / Ctrl+C
       │
       ▼
     Context
       │
       ▼
 parar produção
       │
       ▼
 não pegar novos jobs
       │
       ▼
 terminar jobs atuais
       │
       ▼
     wg.Wait()
       │
       ▼
    shutdown
```

---

## 27. Princípios importantes

### Concorrência precisa ser limitada

Não crie uma goroutine por job sem pensar em limites. Worker pools permitem controlar a concorrência.

### Channels transportam dados e sincronizam goroutines

Um channel não é apenas uma fila; ele também participa da sincronização entre goroutines.

### Não dependa da ordem de execução

A ordem entre goroutines não é garantida.

### Context é cancelamento cooperativo

`context` não mata código. O código precisa observar o contexto ou usar APIs que façam isso.

### Shutdown e job cancellation são conceitos diferentes

O serviço pode parar de aceitar trabalho enquanto permite que uma execução em andamento termine.

### Use o race detector

Para código concorrente:

```bash
go test -race ./...
```

é uma ferramenta essencial.

---

# Etapa 2 concluída

Ao final desta etapa temos:

```text
✅ goroutines
✅ channels
✅ worker pool
✅ WaitGroup
✅ context cancellation
✅ select
✅ graceful shutdown
✅ SIGTERM / Ctrl+C
✅ testes concorrentes
✅ race detector
```

---

# Próxima etapa — Persistência com PostgreSQL

Hoje os jobs existem apenas em memória:

```text
processo morre
    ↓
jobs desaparecem
```

Na próxima etapa vamos evoluir para algo como:

```text
Producer
   │
   ▼
PostgreSQL
   │
   ▼
Scheduler / Workers
```

Assuntos da próxima etapa:

- PostgreSQL
- modelagem da tabela `jobs`
- migrations
- `database/sql`
- connection pool
- repository pattern
- transactions
- índices
- estados do job
- recuperação após restart
- `SELECT ... FOR UPDATE`
- `SKIP LOCKED`
- concorrência entre múltiplas instâncias
- retries e leases
