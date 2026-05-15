# Gemma 4 E2B Fine-tuning Plan for Biqly on macOS

Bu doküman, Biqly'nin mevcut `prompt.go` tabanlı doğal dil -> `LogicalQuery` akışına göre `gemma-4-E2B-it` ailesini fine-tune etmek için izlenecek yolu macOS odaklı olarak tarif eder.

Önemli karar: `gemma-4-E2B-it-Q5_K_M.gguf` dosyasını doğrudan eğitim checkpoint'i gibi kullanmayacağız. GGUF, Biqly'de local inference ve baseline ölçümü için kullanılacak final runtime formatıdır. Fine-tune tarafında Unsloth/Transformers uyumlu safetensors veya Unsloth 4-bit model ile LoRA/QLoRA eğitilir, sonra sonuç tekrar GGUF olarak export edilir.

macOS için kritik ayrım şudur:

```text
macOS = local GGUF inference + eval + dataset preparation
CUDA Linux / Cloud GPU = Unsloth LoRA/QLoRA training + GGUF export
```

Yani Mac üzerinde model çalıştırma, baseline alma, eval koşma ve dataset üretme işleri yapılır. Ancak Unsloth ile LoRA/QLoRA eğitimi için pratik ana yol CUDA destekli Linux/Cloud GPU ortamıdır.

---

## 1. Koddan Çıkan Mevcut AI Sözleşmesi

Biqly'nin üretim akışı şu şekilde:

1. Kullanıcı doğal dilde soru sorar.
2. `PromptBuilder.Build(...)`, semantic model, dimension, metric, join, sample row, few-shot ve conversation history bilgisini tek prompt'a yazar.
3. LLM sadece `LogicalQuery` JSON döndürür.
4. Backend JSON'u parse eder, semantic validator ile denetler, SQL'e compile eder ve gerekirse dry-run/retry yapar.

Fine-tune verisi bu sözleşmeye sadık olmalı. Modelden SQL, açıklama, markdown veya reasoning istemeyeceğiz; sadece geçerli `LogicalQuery` JSON ürettireceğiz.

### `prompt.go` İçindeki Öğrenilmesi Gereken Davranışlar

Dataset aşağıdaki davranışları özellikle kapsamalı:

- Strict JSON: property adları çift tırnaklı, markdown/code fence yok.
- Sadece semantic layer'da listelenen dimension/metric adları kullanılmalı.
- Display dimension önceliği: "müşteri adı", "ürün listesi" gibi sorularda ID yerine `name`, `title`, `label` gibi okunabilir dimension seçilmeli.
- Grouping: "ülkeye göre", "aylık", "bazında", "per customer" gibi ifadelerde dimension hem `select` hem `group_by` içinde olmalı.
- Tarih filtresi: "2026 yılında" gibi ifadeler raw timestamp'e integer filtre yazmamalı; varsa `*_year`, `*_month`, `*_day` grain dimension kullanılmalı.
- Time grain: pre-bucket dimension yoksa raw date dimension `group_by` içinde `time_grain` ile kullanılmalı.
- Ranking: "en yüksek", "top", "most" soruları metric `order_by desc` ve küçük `limit` üretmeli.
- Aggregate threshold: "10'dan fazla siparişi olan müşteriler" gibi sorular `having`, düz `filters` değil.
- Soft delete: "silinen", "deleted", "removed" ifadeleri varsa `deleted_at is_not_null`, `is_deleted = true` veya `delete_flag = 1` gibi mevcut deletion indicator dimension'ına filtre koymalı.
- Prior turns: "bunu geçen aya göre filtrele" gibi takip sorularında önceki `LogicalQuery` dikkate alınmalı.

---

## 2. macOS Rolü ve Sınırları

macOS makine fine-tune eğitim makinesi değil, local inference/eval makinesi olarak kullanılacaktır.

Mac üzerinde yapılacak işler:

- GGUF baseline modeli çalıştırmak
- Biqly'yi OpenAI-compatible local endpoint'e bağlamak
- eval ve smoke test çalıştırmak
- dataset exporter çalıştırmak
- fine-tuned GGUF çıktısını test etmek

Mac üzerinde ana yol olarak yapılmayacak işler:

- Unsloth LoRA/QLoRA training
- Büyük context ile GPU training
- CUDA gerektiren model merge/export pipeline'ı

Eğitim için önerilen ortamlar:

- RunPod
- Lambda Labs
- Vast.ai
- Google Colab
- Kendi CUDA destekli Linux sunucun

---

## 3. macOS Üzerinde llama.cpp Kurulumu

Homebrew ile llama.cpp kurulabilir:

```bash
brew install llama.cpp
```

Kurulumu kontrol et:

```bash
llama-server --help
```

Apple Silicon üzerinde llama.cpp Metal backend ile GGUF inference için uygundur.

---

## 4. Baseline: Mac Üzerinde Gemma 4 E2B GGUF Çalıştırma

Önce `unsloth/gemma-4-E2B-it-GGUF` reposundaki `gemma-4-E2B-it-Q5_K_M.gguf` ile baseline alınır.

Local-only kullanım için `127.0.0.1` tercih edilir:

```bash
llama-server \
  -hf unsloth/gemma-4-E2B-it-GGUF:Q5_K_M \
  --alias gemma-4-e2b-it-q5 \
  --host 127.0.0.1 \
  --port 8001 \
  --temp 0 \
  --top-p 0.95 \
  --ctx-size 32768 \
  --chat-template-kwargs '{"enable_thinking":false}'
```

Alternatif kısa context parametresiyle:

```bash
llama-server \
  -hf unsloth/gemma-4-E2B-it-GGUF:Q5_K_M \
  --alias gemma-4-e2b-it-q5 \
  --host 127.0.0.1 \
  --port 8001 \
  -c 32768 \
  --temp 0 \
  --top-p 0.95 \
  --chat-template-kwargs '{"enable_thinking":false}'
```

Ağdan erişim gerekiyorsa `--host 0.0.0.0` kullanılabilir, fakat local geliştirme için önerilmez.

---

## 5. Biqly macOS `.env` Ayarları

Biqly tarafında local OpenAI-compatible endpoint'e yönlendir:

```env
BI_AI_QUERY_PROVIDER=openai-compatible
BI_AI_QUERY_MODEL=gemma-4-e2b-it-q5
BI_AI_QUERY_BASE_URL=http://127.0.0.1:8001/v1
BI_AI_QUERY_API_KEY=local
BI_AI_TEMPERATURE=0
BI_AI_TOP_P=0.95
BI_AI_MAX_RETRIES=2
BI_AI_MAX_TOKENS=4096
BI_AI_NUM_CTX=32768
BI_AI_MULTI_CANDIDATE_COUNT=1
```

İlk testlerde `BI_AI_MULTI_CANDIDATE_COUNT=1` kalmalı. Local modelde 3 candidate latency'yi ciddi artırabilir.

---

## 6. macOS Baseline Smoke Test

llama-server ayağa kalktıktan sonra modelleri kontrol et:

```bash
curl http://127.0.0.1:8001/v1/models
```

Chat completion testi:

```bash
curl http://127.0.0.1:8001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemma-4-e2b-it-q5",
    "temperature": 0,
    "messages": [
      {
        "role": "system",
        "content": "You are a Business Intelligence query assistant. Output only valid JSON."
      },
      {
        "role": "user",
        "content": "Return a LogicalQuery JSON that counts rows."
      }
    ]
  }'
```

Beklenen davranış: model açıklama, markdown veya code fence değil, doğrudan JSON üretmeli.

---

## 7. Model Stratejisi

### Baseline

Mac üzerinde baseline için GGUF model kullanılır:

```text
unsloth/gemma-4-E2B-it-GGUF:Q5_K_M
```

Bu model eğitim için değil, runtime ve baseline ölçümü içindir.

### Fine-tune İçin Kullanılacak Base

Eğitimde hedef sıralaması:

1. `unsloth/gemma-4-E2B-it-unsloth-bnb-4bit` veya `unsloth/gemma-4-E2B-it`
2. LoRA/QLoRA eğitimi CUDA Linux/Cloud GPU üzerinde yapılır.
3. Eğitim bittiğinde LoRA adapter saklanır.
4. LoRA merge/export sonrası önce `Q8_0`, sonra final runtime için `Q5_K_M` GGUF üretilir.
5. GGUF dosyaları Mac'e indirilir ve llama.cpp ile test edilir.

Bu ayrım kritik:

```text
Q5 GGUF = baseline ve deployment formatı
safetensors / LoRA = eğitim formatı
```

---

## 8. Dataset Kaynakları

Fine-tune dataset'i dört kaynaktan üret:

### 8.1 Curated examples: `few_shot_examples`

En temiz kaynaktır. Her row zaten `question` + validated `logical_query` içerir.

### 8.2 Pozitif geçmiş: `ai_query_history`

Sadece başarılı, çalıştırılmış ve mümkünse `user_rating = 'positive'` olan satırlar alınmalı.

Negatif rating'li kayıtlar eğitime girmemeli; hata analizi ve eval case üretimi için saklanmalı.

### 8.3 Feedback: `ai_feedback`

Pozitif feedback train'e adaydır.

Negatif feedback doğrudan train'e konmamalı; `hard_eval` veya düzeltilecek label backlog'u olarak tutulmalı.

### 8.4 Golden cases: `DefaultGoldenCases()`

Şu an küçük bir setse smoke test için iyi kullanılır.

Train set'e direkt gömmek yerine eval smoke set olarak tutmak daha doğru. Gerekirse farklı varyasyonları train'e eklenebilir.

---

## 9. Eğitim Kaydı Formatı

Üretim çağrısı `Client.GenerateAt(...)` içinde şu chat şekline sahip:

- `system`: `You are a Business Intelligence query assistant. Output only valid JSON.`
- `user`: `PromptBuilder.Build(...)` çıktısı
- `assistant`: raw `LogicalQuery` JSON

Bu yüzden SFT dataset'i aynı formata yakın olmalı:

```jsonl
{"messages":[{"role":"system","content":"You are a Business Intelligence query assistant. Output only valid JSON."},{"role":"user","content":"<PromptBuilder.Build(question, model, ...)>"},{"role":"assistant","content":"{\"select\":[{\"type\":\"metric\",\"name\":\"row_count\"}],\"limit\":100}"}],"text":"<tokenizer chat-template uygulanmış eğitim metni>"}
```

Veri üretiminde assistant cevabı normalize edilmeli:

- JSON minified olmalı.
- Exporter, `messages` yanında tokenizer chat template uygulanmış `text` alanı da üretmeli.
- Training job bu `text` alanını okumalı.
- `limit` yoksa Biqly varsayılanına göre `100` eklenmeli.
- `version`, `datasource_id`, `model_id` gibi runtime'a göre değişen alanlar sadece üretim prompt'unda gerçekten bekleniyorsa tutulmalı.
- Genel eğitimde modelin schema-id ezberlemesini önlemek için mümkün olduğunca runtime-specific ID'ler çıkarılmalı.
- Assistant cevabında açıklama, markdown, code block veya reasoning olmamalı.

---

## 10. Dataset Çıkarma Adımları

### 10.1 SQL ile Aday Kayıtları Çek

Curated examples:

```sql
SELECT
  'few_shot' AS source,
  question,
  logical_query,
  datasource_id,
  model_id,
  dialect,
  tags,
  updated_at
FROM few_shot_examples
WHERE question <> ''
  AND logical_query IS NOT NULL;
```

Başarılı ve pozitif AI geçmişi:

```sql
SELECT
  'history_positive' AS source,
  question,
  logical_query,
  datasource_id,
  model_id,
  model_used,
  created_at
FROM ai_query_history
WHERE status = 'success'
  AND logical_query IS NOT NULL
  AND (user_rating = 'positive' OR user_rating IS NULL);
```

Negatif feedback için ayrı hard-eval listesi oluştur:

```sql
SELECT
  question,
  datasource_id,
  categories,
  feedback_text,
  created_at
FROM ai_feedback
WHERE rating = 'negative'
ORDER BY created_at DESC;
```

### 10.2 Prompt'u Kod ile Yeniden Üret

En doğru yöntem, SQL'den gelen kayıt için semantic model'i repository'den yükleyip `PromptBuilder.Build(...)` çağıran küçük bir Go exporter yazmaktır.

Önerilen yapı:

```text
cmd/export-sft/main.go
  - few_shot_examples + positive ai_query_history okur
  - semantic model'i datasource/model_id ile yükler
  - PromptBuilder.Build(question, model, maxPromptRunes, examples, samples, priorTurns, deniedFields) çağırır
  - assistant cevabını canonical JSON'a çevirir
  - train.jsonl / validation.jsonl / hard_eval.jsonl yazar
```

Exporter üretimdeki prompt path'ini kullandığı için fine-tune, gerçek Biqly prompt dağılımını öğrenir.

### 10.3 Train / Validation / Hard Eval Split

Önerilen split:

- `train.jsonl`: %80, pozitif ve temiz örnekler.
- `validation.jsonl`: %10, aynı datasource/model dağılımından ama train'e girmeyen örnekler.
- `hard_eval.jsonl`: %10 + negatif feedback'ten elle düzeltilmiş zor sorular.

Aynı question'ın küçük varyasyonları aynı split'te kalmalı; aksi halde validation skoru şişer.

---

## 11. Dataset Kapsam Kontrol Listesi

İlk gerçek fine-tune için minimum hedef:

- En az 300-500 temiz question -> LogicalQuery çifti.
- Türkçe ve İngilizce sorular birlikte.
- Her aktif datasource/semantic model için örnek.
- En az 50 grouping örneği.
- En az 30 tarih filtresi ve time grain örneği.
- En az 30 ranking/top-N örneği.
- En az 20 `having` örneği.
- En az 20 soft-delete örneği.
- En az 20 follow-up/prior-turn örneği.
- En az 50 "ID yerine display dimension" örneği.

Veri azsa önce fine-tune'a koşmak yerine `few_shot_examples` ve golden eval setini büyütmek daha yüksek getirili olur.

---

## 12. Mac -> Cloud GPU Çalışma Akışı

### 12.1 Mac üzerinde dataset üret

```bash
go run ./cmd/export-sft \
  -out data/biqly-gemma4 \
  -train-ratio 0.8 \
  -validation-ratio 0.1
```

Beklenen çıktı:

```text
data/biqly-gemma4/train.jsonl
data/biqly-gemma4/validation.jsonl
data/biqly-gemma4/hard_eval.jsonl
```

### 12.2 Dataset'i Cloud GPU makinesine gönder

```bash
rsync -avz data/biqly-gemma4/ user@gpu-host:/workspace/biqly/data/biqly-gemma4/
```

### 12.3 Cloud GPU üzerinde eğitim yap

Cloud GPU ortamında Unsloth LoRA/QLoRA eğitimi çalıştırılır.

### 12.4 GGUF çıktısını Mac'e indir

```bash
rsync -avz user@gpu-host:/workspace/biqly/outputs/biqly-gemma4-e2b-gguf-q5/ ./outputs/
```

### 12.5 Mac üzerinde fine-tuned GGUF test et

```bash
llama-server \
  --model ./outputs/biqly-gemma4-e2b-gguf-q5/biqly-gemma4-e2b.Q5_K_M.gguf \
  --alias biqly-gemma4-e2b-q5 \
  --host 127.0.0.1 \
  --port 8001 \
  --temp 0 \
  --top-p 0.95 \
  --ctx-size 32768 \
  --chat-template-kwargs '{"enable_thinking":false}'
```

---

## 13. Unsloth SFT/LoRA Eğitimi - CUDA Linux Ortamı

Bu adım macOS üzerinde ana yol olarak çalıştırılmayacaktır. Eğitim için CUDA destekli Linux/Cloud GPU ortamı kullanılacaktır.

Başlangıç için LoRA/QLoRA:

```python
from unsloth import FastLanguageModel
from datasets import load_dataset
from trl import SFTTrainer, SFTConfig

max_seq_length = 8192  # once stable, test 16384/32768

dataset = load_dataset(
    "json",
    data_files={
        "train": "data/biqly-gemma4/train.jsonl",
        "validation": "data/biqly-gemma4/validation.jsonl",
    },
)

model, tokenizer = FastLanguageModel.from_pretrained(
    model_name="unsloth/gemma-4-E2B-it-unsloth-bnb-4bit",
    max_seq_length=max_seq_length,
    load_in_4bit=True,
)

model = FastLanguageModel.get_peft_model(
    model,
    r=16,
    target_modules=[
        "q_proj", "k_proj", "v_proj", "o_proj",
        "gate_proj", "up_proj", "down_proj",
    ],
    lora_alpha=16,
    lora_dropout=0,
    bias="none",
    use_gradient_checkpointing="unsloth",
    random_state=3407,
    max_seq_length=max_seq_length,
)

trainer = SFTTrainer(
    model=model,
    tokenizer=tokenizer,
    train_dataset=dataset["train"],
    eval_dataset=dataset["validation"],
    args=SFTConfig(
        dataset_text_field="text",
        max_seq_length=max_seq_length,
        per_device_train_batch_size=1,
        gradient_accumulation_steps=8,
        learning_rate=2e-4,
        warmup_ratio=0.05,
        num_train_epochs=2,
        logging_steps=10,
        eval_steps=100,
        save_steps=100,
        output_dir="outputs/biqly-gemma4-e2b-lora",
        optim="adamw_8bit",
        seed=3407,
    ),
)

trainer.train()
model.save_pretrained("outputs/biqly-gemma4-e2b-lora")
tokenizer.save_pretrained("outputs/biqly-gemma4-e2b-lora")
```

Notlar:

- SFT dataset `messages` formatını saklayabilir, fakat training için tokenizer chat template uygulanmış tek `text` alanı kullanılmalı.
- Bu `text` alanını exporter'da üretmek daha deterministik olur.
- Gemma 4 thinking mode production JSON çıkışı için kapalı tutulmalı.
- Dataset'te thought/reasoning bloğu olmamalı.
- İlk koşuda `max_seq_length=8192` ile pipeline doğrulanmalı.
- Semantic model prompt'ları uzunsa 16K/32K test edilmeli.

---

## 14. Export: LoRA -> GGUF

Önce kalite kaybını ayırmak için `Q8_0` export al, sonra hedef `Q5_K_M` üret:

```python
model.save_pretrained_gguf(
    "outputs/biqly-gemma4-e2b-gguf",
    tokenizer,
    quantization_method="q8_0",
)

model.save_pretrained_gguf(
    "outputs/biqly-gemma4-e2b-gguf-q5",
    tokenizer,
    quantization_method="q5_k_m",
)
```

Eğer Q8 iyi ama Q5 kötüleşirse sorun fine-tune değil quantization kaybıdır. Bu durumda runtime için Q8 veya Unsloth Dynamic quant denenmeli.

---

## 15. Fine-tuned GGUF ile Biqly Entegrasyonu

Fine-tuned GGUF'u Mac üzerinde llama.cpp ile servis et:

```bash
llama-server \
  --model ./outputs/biqly-gemma4-e2b-gguf-q5/biqly-gemma4-e2b.Q5_K_M.gguf \
  --alias biqly-gemma4-e2b-q5 \
  --host 127.0.0.1 \
  --port 8001 \
  --temp 0 \
  --top-p 0.95 \
  --ctx-size 32768 \
  --chat-template-kwargs '{"enable_thinking":false}'
```

Biqly env:

```env
BI_AI_QUERY_PROVIDER=openai-compatible
BI_AI_QUERY_MODEL=biqly-gemma4-e2b-q5
BI_AI_QUERY_BASE_URL=http://127.0.0.1:8001/v1
BI_AI_QUERY_API_KEY=local
BI_AI_TEMPERATURE=0
BI_AI_TOP_P=0.95
BI_AI_NUM_CTX=32768
BI_AI_MAX_TOKENS=4096
BI_AI_MAX_RETRIES=2
BI_AI_MULTI_CANDIDATE_COUNT=1
```

Self-consistency kalite/latency tradeoff testi için:

```env
BI_AI_MULTI_CANDIDATE_COUNT=3
```

Ancak Mac üzerinde önce `1` ile ölçüm yapılmalı.

---

## 16. macOS Performans Notları

### 16.1 Context boyutu

Başlangıç:

```bash
--ctx-size 32768
```

RAM baskısı veya yavaşlama olursa:

```bash
--ctx-size 16384
```

Semantic catalog çok büyükse ve prompt kesiliyorsa:

```bash
--ctx-size 65536
```

64K her Mac'te pratik olmayabilir. Önce 32K ile baseline alınmalı.

### 16.2 Host güvenliği

Local testte:

```bash
--host 127.0.0.1
```

Ağdan erişim gerekiyorsa:

```bash
--host 0.0.0.0
```

Bu durumda firewall veya reverse proxy ile korumak gerekir.

### 16.3 Q5 mi Q8 mi?

Mac'te test sırası şöyle olsun:

```text
1. Baseline Q5_K_M
2. Fine-tuned Q8_0
3. Fine-tuned Q5_K_M
```

Q8 daha kaliteli ama daha fazla RAM kullanır. Q5 daha uygun runtime formatıdır.

### 16.4 JSON için sıcaklık

Biqly text-to-LogicalQuery akışında:

```env
BI_AI_TEMPERATURE=0
```

Bu doğru ayardır. Bu tip görevde yaratıcılık değil deterministik JSON üretimi gerekir.

---

## 17. Eval Gate

Her model için aynı sırayla ölç:

1. Baseline OpenAI/Anthropic mevcut model.
2. `gemma-4-E2B-it-Q5_K_M.gguf` zero/few-shot baseline.
3. Fine-tuned Q8 GGUF.
4. Fine-tuned Q5_K_M GGUF.

Kabul kriterleri:

- Built-in eval pass rate baseline'dan düşük olmamalı.
- `hard_eval.jsonl` üzerinde en az %15-20 iyileşme hedeflenmeli.
- JSON parse failure oranı %1'in altında olmalı.
- Semantic validation failure oranı baseline'dan düşük olmalı.
- Median latency local endpoint için kabul edilebilir sınırda olmalı.

Biqly API ile smoke:

```bash
curl -X POST http://localhost:8888/api/ai/eval/run \
  -H "Authorization: Bearer $BI_ADMIN_API_KEY"
```

Ek olarak exporter'ın ürettiği hard eval seti için küçük bir CLI veya test eklenmeli:

```text
go test ./internal/ai -run TestLiveGemmaFineTuneHardEval -count=1
```

Bu test `BI_AI_GOLDEN_EVAL=1` gibi explicit flag olmadan çalışmamalı.

---

## 18. Fine-tune Sonrası Hata Analizi

Her failed case için şunları sınıflandır:

- Parse error: JSON bozuk, açıklama eklemiş, code fence koymuş.
- Field hallucination: semantic model'de olmayan field kullanmış.
- Wrong display dimension: ID seçmiş, human-readable dimension seçmemiş.
- Wrong time filter: raw date'e integer yıl/ay filtrelemiş.
- Missing group_by select: group_by var ama period/category select'te yok.
- Filter vs having karışmış.
- Soft-delete filtresi eksik.
- Follow-up bağlamını yanlış taşımış.

Her sınıftan 10-20 düzeltilmiş örnek train'e geri döndürülür. Bu döngü, tek seferlik büyük eğitimden daha iyi sonuç verir.

---

## 19. macOS Uygulama Sırası

- [ ] macOS üzerinde `brew install llama.cpp` ile llama.cpp kur.
- [ ] `llama-server --help` ile kurulum kontrolü yap.
- [ ] `gemma-4-E2B-it-Q5_K_M.gguf` baseline modeli `llama-server` ile çalıştır.
- [ ] Biqly `BI_AI_QUERY_*` env değerlerini `http://127.0.0.1:8001/v1` endpoint'ine yönlendir.
- [ ] `/v1/models` ve `/v1/chat/completions` ile endpoint smoke test yap.
- [ ] Biqly built-in eval + 20 gerçek soru ile baseline skor al.
- [ ] Mac üzerinde `cmd/export-sft` ile `train.jsonl`, `validation.jsonl`, `hard_eval.jsonl` üret.
- [ ] Dataset kalite kontrolü yap: duplicate, invalid JSON, hallucinated field, train/validation leak.
- [ ] Dataset'i CUDA destekli Linux/Cloud GPU ortamına gönder.
- [ ] Unsloth LoRA/QLoRA eğitimini Cloud GPU üzerinde çalıştır.
- [ ] Cloud GPU üzerinde Q8_0 ve Q5_K_M GGUF export al.
- [ ] Fine-tuned GGUF dosyalarını Mac'e indir.
- [ ] Mac üzerinde Q8_0 ve Q5_K_M modellerini ayrı ayrı `llama-server` ile servis et.
- [ ] Biqly eval sonuçlarını baseline ile karşılaştır.
- [ ] Hata sınıflarını etiketle, dataset'e düzeltme ekle, ikinci fine-tune döngüsüne geç.
- [ ] Skor kabul kriterini geçerse modeli version'la: `biqly-gemma4-e2b-lora-vYYYYMMDD`.

---

## 20. Dosya ve Repo Önerisi

Training artifact'lerini app repo'suna büyük dosya olarak koyma:

```text
biqly/
  docs/gemma-4-macos-fine-tuning-plan.md
  tools/export-sft/              # ileride eklenebilir
  evalsets/
    hard_eval.example.jsonl      # küçük örnek set olabilir

external/private artifact storage:
  data/biqly-gemma4/train.jsonl
  data/biqly-gemma4/validation.jsonl
  data/biqly-gemma4/hard_eval.jsonl
  outputs/biqly-gemma4-e2b-lora/
  outputs/biqly-gemma4-e2b-gguf/
  outputs/biqly-gemma4-e2b-gguf-q5/
```

---

## 21. Kısa Özet

Mevcut planın mantıklı, fakat macOS için merkez karar şu olmalı:

```text
MacOS = dataset export + local GGUF inference + eval
Cloud GPU = Unsloth LoRA/QLoRA training + GGUF export
```

`gemma-4-E2B-it-Q5_K_M.gguf` dosyasını Mac'te baseline ve final runtime için kullanabilirsin. LoRA fine-tune'u ise CUDA destekli bir ortamda koşturup çıkan GGUF'u tekrar Mac'e indirerek Biqly ile test etmelisin.

