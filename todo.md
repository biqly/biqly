# Biqly WrenAI Benzeri Calisma Mantigi TODO

Bu TODO'nun kapsami net: Biqly **Go tabanli kalacak**. WrenAI'den Rust,
DataFusion, Python SDK, WASM, framework adapter ya da birebir repo yapisi
alinmayacak. Sadece Biqly'nin mevcut `LogicalQuery`-first mimarisine mantikli
olan calisma prensipleri uygulanacak.

## Alinacak Prensipler

- **Raw schema degil, context layer:** AI ve kullanici ham tablo/kolon kaosu
  yerine is anlamlari, iliskiler, metrikler ve kurallar ile tanimli bir semantic
  context uzerinden calismali.
- **Tek dogru semantic tanim:** UI, API, query compiler, AI prompt, validation ve
  eval ayni model/metric/join/policy tanimlarini kullanmali.
- **Query dogrulugu prompt'a birakilmamali:** Retrieval, relationship metadata,
  validation, dry-run, retry ve feedback birlikte calismali.
- **Degisiklikler deploy mantigiyla aktif olmali:** Semantic model uzerindeki
  kritik degisiklikler validate edilmeden query runtime'a sizmamali.
- **Feedback sisteme geri donmeli:** Basarili sorgular ve kullanici feedback'i
  few-shot, eval ve context secimi icin kullanilmali.
- **Governance context'in parcasi olmali:** Model-level, field-level ve row-level
  izinler semantic layer ile birlikte dusunulmeli.

## Alinmayacaklar

- [x] Go disinda yeni engine stack'e gecmek.
- [x] Rust/DataFusion benzeri ayri query engine yazmak.
- [x] Python/JS SDK veya WASM hedefini ilk faza almak.
- [x] AI'ya raw SQL urettirmek.
- [x] WrenAI dosya yapisini birebir kopyalamak.
- [x] Agent skill/MCP/SDK gibi entegrasyonlari core davranis oturmadan yapmak.

## Biqly'de Korunacak Temel Kararlar

- [x] AI raw SQL degil, `LogicalQuery` JSON uretir.
- [x] SQL'i backend compiler uretir.
- [x] Query compiler dialect-aware kalir.
- [x] Metadata DB, semantic model, dimensions, metrics, joins ve query history
  mevcut kaynak olarak kullanilir.
- [x] PostgreSQL, MySQL, SQL Server ve ClickHouse driver/dialect yapisi korunur.
- [x] Dry-run/EXPLAIN, retry, few-shot, sample data, prior turns ve embedding
  tabanli routing mevcut altyapinin uzerine gelistirilir.

## Hedef Calisma Mantigi

```text
Datasource metadata sync
        |
        v
Semantic context olustur / guncelle
        |
        v
Context validate et ve aktif versiyon olarak yayinla
        |
        v
User question
        |
        v
Context retrieval: ilgili model, kolon, metric, join, policy
        |
        v
AI -> LogicalQuery JSON
        |
        v
Semantic validate + permission validate + dry-run
        |
        v
Compile + execute + audit + feedback
        |
        v
Basarili/hatali sonuc eval ve few-shot dongusunu besler
```

## P0 - Semantic Context'i Urunun Merkezi Yap

- [x] Mevcut `semantic_models`, `semantic_dimensions`, `semantic_metrics` ve
  `semantic_joins` yapilarini "aktif semantic context" olarak adlandir.
- [x] Her AI query response'unda kullanilan context bilgisini dondur:
  - selected models/tables
  - selected dimensions/metrics
  - join paths
  - routing confidence
  - ranking method
  - context version veya updated timestamp
- [x] `TableRouter` sonucunu debug edilebilir hale getir:
  - keyword score
  - embedding score
  - relation expansion
  - bridge table secimi
  - elenen adaylar
- [x] Semantic model olmayan ham tablolar icin auto-context uretimi devam etsin,
  fakat kalici model varsa once o kullanilsin.
- [x] AI prompt'u sadece gerekli context ile beslensin; buyuk semalarda tum
  katalog prompt'a tasinmasin.

Kabul kriteri:

- Bir AI sorgusunda hangi context'in neden secildigi API response ve UI'da
  gorulebilir.
- Yanlis cevapta hata retrieval mi, prompt mu, validation mi anlasilabilir.

## P0 - Semantic Degisikliklerde Validate/Publish Mantigi

- [x] Semantic model degisiklikleri icin `draft -> validate -> publish` akisi
  tasarla.
- [x] Publish oncesi kontroller:
  - duplicate model/field/relationship var mi
  - join condition metadata ile uyumlu mu
  - metric expression mevcut kolonlara referans veriyor mu
  - relationship cardinality aggregation icin riskli mi
  - prompt/context boyutu makul mu
  - permission policy referanslari geciyor mu
- [x] Aktif semantic context icin `version`, `published_at`, `published_by`
  alanlari eklemeyi degerlendir.
- [x] Publish edilmemis degisikliklerin query runtime'a etki etmemesini sagla.
- [x] Rollback icin onceki aktif context snapshot'ini saklama stratejisi belirle.

Kabul kriteri:

- Hatali semantic join veya metric publish asamasinda yakalanir.
- Kullanici publish etmeden production query davranisi degismez.

## P1 - Go Icindeki Core Sınırı Netlestir

- [x] API handler'larin icindeki ortak query akisini Go servis katmanina tasarla:
  - context load
  - route
  - AI generate
  - validate
  - dry-run
  - compile
  - execute
  - audit/history
- [x] `internal/query`, `internal/semantic`, `internal/ai`, `internal/security`
  arasindaki sorumluluklari netlestir.
- [x] API endpoint'leri ve ileride yazilacak worker/CLI ayni Go servis akisini
  kullanacak sekilde konumlandir.
- [x] `LogicalQuery` schema, Go struct ve frontend type'lari arasinda contract
  test ekle.
- [x] Golden SQL testlerini semantic context fixture'lari ile guclendir.

Kabul kriteri:

- Ayni `LogicalQuery` ve ayni semantic context API, test ve varsa CLI path'inde
  ayni SQL'i uretir.

## P1 - Relationship ve Metric Kalitesini Artir

- [x] Relationship metadata'yi sadece join listesi olarak degil, query planning
  sinyali olarak kullan:
  - one_to_one
  - many_to_one
  - one_to_many
  - many_to_many
- [x] Aggregation fanout risklerini validator'da daha net yakala.
- [x] Display/name kolonlarini identifier kolonlarina gore daha yuksek oncelikli
  hale getir.
- [x] Calculated field ihtiyacini Go compiler icinde kontrollu sekilde tasarla:
  - izinli expression listesi
  - semantic validation
  - dialect compile
  - test fixture
- [x] Reusable metric tanimlarini prompt, validator ve compiler tarafinda tek
  kaynaktan kullandir.

Kabul kriteri:

- AI "musteri bazinda ciro" gibi sorularda id kolonlari yerine okunabilir
  dimension'lari tercih eder.
- Riskli join/aggregation durumlari sessizce yanlis sonuc uretmez.

## P1 - Retrieval ve Ambiguity Akisini Guclendir

- [x] Metadata embedding refresh islemini manuel endpoint disinda metadata sync
  sonrasi opsiyonel Go job olarak calistir.
- [x] Tablo secimi dusuk confidence ise direkt sorgu uretmek yerine netlestirme
  sorusu dondur.
- [x] Netlestirme sorularini sadece "hangi tablo" seviyesinde birakma; is
  anlamina gore uret:
  - customer mi reseller mi employee mi
  - order date mi ship date mi
  - revenue gross mu net mi
- [x] Column selection'i table selection'dan ayri skorla.
- [x] Prompt'a sadece secilen kolonlari koy; elenen kolonlari debug trace'e yaz.

Kabul kriteri:

- Ambiguous sorularda sistem uydurma query yerine kullanicidan net bilgi ister.
- Buyuk semalarda prompt sismez, fakat gerekli kolonlar kaybolmaz.

## P1 - Feedback ve Few-shot Dongusu

- [x] Positive feedback alan basarili sorgulari few-shot candidate olarak isaretle.
- [x] Negative feedback kategorilerini standartlastir:
  - wrong_table
  - wrong_join
  - wrong_metric
  - wrong_filter
  - wrong_date_range
  - execution_error
  - unclear_question
- [x] Few-shot secimi sadece en yeni kayitlardan degil, benzer soru + ayni
  datasource/model + positive feedback sinyallerinden gelsin.
- [x] Feedback kaydi AI history ile iliskilendirilsin; sadece serbest text olarak
  kalmasin.
- [x] Dashboard'da model bazli success/failure oranlari gorunsun.

Kabul kriteri:

- Kullanici feedback'i sonraki benzer sorularin context/few-shot secimini
  iyilestirir.

## P1.5 - Composite Semantic Models  *(DEFERRED — Kasim 2026 review)*

> Not: Composite model ozelligi, WrenAI'deki semantic context yaklasimindan
> ilham alir; ancak Biqly'de Go + LogicalQuery-first mimari korunur. Composite
> model bir SQL view veya ayri query engine degildir. Backend compiler ve
> validator akisi ayni kalir.
>
> Composite model en dogru yerde semantic layer ile query planner arasina
> kurgulanir. Yani raw metadata tarafinda degil, AI prompt icinde rastgele
> degil, SQL compiler icinde de sonradan uydurularak degil.
>
> **Review karari:** Composite model fikri dogru ama LogicalQuery, validator,
> planner, prompt builder, TableRouter, permission, audit, query history, UI ve
> testlerin hepsine dokundugu icin erken eklemek silent regression riski uretir.
> Base semantic model + AI flow + router taş gibi calistiktan sonra
> (MVP E asamasinda) acilacak.

Composite model, birden fazla base semantic modelin kontrollu sekilde bir araya
getirilmesiyle olusan business-level virtual dataset olarak kullanilacak.

Amac; AI'nin ham tablolar veya rastgele join path'ler uzerinden calismasi yerine,
oncedan tanimlanmis, validate edilmis ve publish edilmis is baglamlari uzerinden
`LogicalQuery` uretmesini saglamak.

Ornek composite modeller:

- `sales_performance` -> orders, order_items, customers, products, sales_reps
- `customer_360` -> customers, orders, invoices, payments, support_tickets
- `finance_overview` -> invoices, payments, customers, currencies

### Alinacak Temel Kararlar

- [ ] Composite model raw SQL view degildir.
- [ ] Composite model fiziksel tablo degildir.
- [ ] Composite model semantic layer icinde tanimlanan published business dataset'tir.
- [ ] AI yine raw SQL uretmez.
- [ ] AI, composite model context'i uzerinden `LogicalQuery` JSON uretir.
- [ ] SQL uretimi yine backend compiler tarafindan yapilir.
- [ ] Composite model sadece publish edilmis semantic context uzerinden runtime'a
      dahil olur.

### LogicalQuery Genisletmesi

- [ ] `composite_model_id` alani ekle; `model_id` ile mutual exclusion.
- [ ] Ikisi birden doluysa query reject.
- [ ] Ikisi de bossa TableRouter karar verir.
- [ ] `composite_model_id` doluysa select/filter/group/order sadece composite
      modelde expose edilmis field'lari kullanabilir.
- [ ] Composite model publish edilmemisse query reject.

```go
type LogicalQuery struct {
    DatasourceID     string       `json:"datasource_id"`
    ModelID          string       `json:"model_id,omitempty"`
    CompositeModelID string       `json:"composite_model_id,omitempty"`
    Select           []SelectItem `json:"select"`
    Filters          []Filter     `json:"filters"`
    GroupBy          []GroupBy    `json:"group_by"`
    OrderBy          []OrderBy    `json:"order_by"`
    Limit            int          `json:"limit"`
    Offset           int          `json:"offset"`
}
```

### Yeni Semantic Yapilar

- [ ] `semantic_composite_models` tablosu: id, datasource_id, name, label,
      description, version, status (draft/published/archived), published_at,
      published_by.
- [ ] `semantic_composite_model_members` tablosu: composite_model_id,
      semantic_model_id, role (base/related/optional), alias, required.
- [ ] `semantic_composite_fields` tablosu: composite_model_id, source_model_id,
      source_field_name, field_type (dimension/metric), exposed_name, label,
      description, is_default.
- [ ] `semantic_composite_joins` tablosu: composite_model_id, from_model_id,
      to_model_id, join_name, join_type (left/inner/right), relationship
      (one_to_one/many_to_one/one_to_many/many_to_many), required, risk_level.

Go dosya yapisi: `internal/semantic/`
- [ ] `composite_model.go`
- [ ] `composite_repository.go`
- [ ] `composite_resolver.go`
- [ ] `composite_validator.go`

### Composite Resolver

- [ ] Composite modelin publish edilmis aktif versiyonunu yukle.
- [ ] Member semantic modelleri coz.
- [ ] Expose edilmis dimension ve metric listesini uret.
- [ ] Source field ile exposed field mapping'ini coz.
- [ ] Approved join path listesini query planner'a ver.
- [ ] Field-level permission kontrolunden gecmis context uret.
- [ ] AI prompt'a sadece composite modelin izin verdigi alanlari gonder.

Ornek mapping:

```text
orders.total_revenue    -> total_revenue
customers.country       -> customer_country
products.category       -> product_category
sales_reps.name         -> sales_rep_name
orders.created_at       -> order_date
```

### Query Planner Entegrasyonu

- [ ] Composite modelde secilen field'lara gore gerekli base modelleri belirle.
- [ ] Sadece composite modelde tanimli approved join path'leri kullan.
- [ ] Join path eksikse query reject.
- [ ] Relationship cardinality kontrolu yap.
- [ ] Aggregation fanout riski varsa warning veya validation error don.
- [ ] Many-to-many join durumlarinda metric aggregation guvenli degilse query
      calistirma.
- [ ] Required join'ler otomatik plana dahil.
- [ ] Optional join'ler sadece ihtiyac varsa plana ekle.

### AI Context Entegrasyonu

- [ ] AI prompt context'e composite modeller dahil et.
- [ ] AI'ya ham tablo kaosu yerine composite model context ver:

```json
{
  "type": "composite_model",
  "name": "sales_performance",
  "description": "Sales, customer, product and revenue analysis context",
  "dimensions": ["customer_name", "customer_country", "product_category",
                  "sales_rep_name", "order_date"],
  "metrics": ["total_revenue", "order_count", "average_order_value"],
  "allowed_filters": ["order_date", "customer_country", "product_category",
                       "sales_rep_name"]
}
```

- [ ] AI composite model context disindaki field'lari goremez.
- [ ] AI composite model disindaki tablo/kolon isimlerini kullanamaz.
- [ ] Composite context dusuk confidence ile secildiyse clarification question
      don.
- [ ] Kullanilan composite model response icinde debug trace olarak goster.

### TableRouter / Context Retrieval Entegrasyonu

- [ ] TableRouter sadece physical table secen yapi olmaktan cikip semantic
      context sececek hale getir.
- [ ] Router aday tipleri: base semantic model, composite semantic model, raw
      auto-context table group.
- [ ] Oncelik sirasi: published composite -> published base -> auto-generated
      raw.
- [ ] Routing response'a `selected_context_type`, `selected_context_name`,
      `selected_models`, `join_paths`, `confidence`, `ranking_method` ekle.

### Validation / Publish Kurallari

Publish oncesi kontroller:

- [ ] Referans verilen base semantic modeller mevcut mu?
- [ ] Base semantic modeller published durumda mi?
- [ ] Expose edilen field'lar gercekten source modelde var mi?
- [ ] Exposed field isimleri duplicate mi?
- [ ] Metric expression referanslari gecerli mi?
- [ ] Join path'ler mevcut semantic join metadata ile uyumlu mu?
- [ ] Relationship cardinality aggregation acisindan riskli mi?
- [ ] Many-to-many iliski varsa metric aggregation guvenli mi?
- [ ] Permission policy referanslari gecerli mi?
- [ ] Prompt/context size makul mu?
- [ ] Composite model en az bir dimension veya metric expose ediyor mu?

Runtime kurallari:

- [ ] Draft composite model query runtime'a etki etmez.
- [ ] Sadece published composite model kullanilabilir.
- [ ] Published composite model version bilgisi query history ve audit log'a yazilir.
- [ ] Hatali composite join veya metric publish asamasinda yakalanir.
- [ ] Rollback icin onceki published version saklanir.

### Permission / Governance

- [ ] Kullanicinin composite model erisimi yoksa context retrieval sonucunda bu
      model donmez.
- [ ] Kullanicinin erisemedigi base model varsa composite model kullanilamaz veya
      ilgili field'lar cikarilir.
- [ ] Field-level permission composite exposed field seviyesinde uygulanir.
- [ ] Permission disi field AI prompt'a hic girmez.
- [ ] Row-level filters compiler'a guvenli sekilde inject edilir.
- [ ] Audit log: composite_model_id, composite_model_version, selected_base_models,
      selected_fields, selected_join_paths, permission_decision, query_fingerprint.

### API Endpoint'ler

- [ ] `POST /api/semantic/composite-models`
- [ ] `GET /api/semantic/composite-models`
- [ ] `GET /api/semantic/composite-models/{id}`
- [ ] `PUT /api/semantic/composite-models/{id}`
- [ ] `DELETE /api/semantic/composite-models/{id}`
- [ ] `POST /api/semantic/composite-models/{id}/members`
- [ ] `POST /api/semantic/composite-models/{id}/fields`
- [ ] `POST /api/semantic/composite-models/{id}/joins`
- [ ] `POST /api/semantic/composite-models/{id}/validate`
- [ ] `POST /api/semantic/composite-models/{id}/publish`
- [ ] `POST /api/semantic/composite-models/{id}/rollback`

### UI

- [ ] Composite model listesi.
- [ ] Composite model detail ekrani: members, exposed fields, approved joins,
      relationship risk, prompt/context size estimate.
- [ ] Draft / published / archived status gostergesi.
- [ ] Validate ve publish butonlari.
- [ ] AI Query ekraninda secilen context'in composite model olup olmadiginin
      gosterilmesi.
- [ ] Context trace icinde composite model, base modeller, selected fields ve
      join path'lerin gosterilmesi.

### Test Stratejisi

Unit testler:

- [ ] Composite model resolver testleri.
- [ ] Composite field mapping testleri.
- [ ] Composite join validation testleri.
- [ ] Duplicate exposed field validation testleri.
- [ ] `model_id` ve `composite_model_id` conflict validation testleri.
- [ ] Permission disi field'in prompt context'e girmedigi testi.
- [ ] Fanout risk validator testleri.

Golden SQL testleri:

- [ ] Composite model ile simple select.
- [ ] Composite model ile dimension + metric group by.
- [ ] Composite model ile filter.
- [ ] Composite model ile order by metric.
- [ ] Composite model ile multi-join query.
- [ ] Riskli many-to-many aggregation rejection.

Eval testleri:

- [ ] "monthly revenue by customer country" -> sales_performance composite model.
- [ ] "top products by revenue" -> products + orders path.
- [ ] "customer payment status" -> customer_360 veya finance context.
- [ ] Ambiguous context -> clarification question.

### Kabul Kriterleri

- Kullanici "monthly revenue by customer country" dediginde sistem published
  composite model uzerinden dogru context'i secer.
- AI sadece composite modelde expose edilen field'lari gorur.
- AI raw SQL degil, LogicalQuery JSON uretir.
- SQL compiler sadece approved join path'leri kullanir.
- Fanout riski olan composite join'ler sessizce yanlis sonuc uretmez.
- Permission disi alan prompt'a girmez, validate edilmez ve compile edilemez.
- Query history ve audit log icinde composite model version bilgisi gorulebilir.
- Composite model publish edilmeden runtime davranisini degistirmez.
- Rollback ile onceki published composite context'e donulebilir.

## P2 - Governance ve Guvenlik

- [x] Datasource DSN encryption eksigini kapat.
- [x] Permission policy'nin AI, QueryBuilder ve direkt query endpoint'lerinde ayni
  sonucu verdigini test et.
- [x] Row-level filters'in compiler'a guvenli sekilde inject edildigini golden
  testlerle dogrula.
- [x] Strict mode ekle:
  - undeclared model/field kullanimi yok
  - raw SQL endpoint'leri admin/debug disinda yok
  - permission disi field prompt'a bile girmemeli
- [x] Audit log'a su alanlari ekle:
  - context version
  - selected model/table
  - selected fields
  - permission decision
  - query fingerprint

Kabul kriteri:

- AI permission disi alani gormez, uretemez, compile ettiremez.

## P2 - Evaluation Sistemi

- [x] `examples/adventureworks` icin golden question set olustur.
- [x] Her test case su alanlari icersin:
  - question
  - expected selected tables
  - expected dimensions/metrics
  - expected LogicalQuery shape
  - expected SQL fragment veya normalized SQL
  - expected columns
- [x] Go test veya mevcut eval endpoint'i ile bu set calissin.
- [x] Context degisikliklerinde regression raporu uret.
- [x] Eval sonucunu provider/model/context version bazinda sakla.

Kabul kriteri:

- Semantic context degisince AI dogrulugu bozulduysa testte gorulur.

## P2 - UI Calisma Prensibi

- [x] Metadata UI'da raw catalog ile business semantic context ayrilsin.
- [x] Semantic model ekraninda:
  - model fields
  - metrics
  - joins
  - relationship riskleri
  - prompt/context size estimate
  gorunsun.
- [x] AI Query ekraninda context trace gorunur olsun.
- [x] Feedback UI standart kategorilerle backend'e baglansin.
- [x] Publish edilmemis semantic degisiklik UI'da acikca isaretlensin.

Kabul kriteri:

- Kullanici yanlis cevabi debug ederken hangi context kullanildiğini UI'da
  gorebilir.

## Ilk Uygulama Sirasi

P0/P1 maddeleri tamamlandi:

1. [x] `TableRouter` debug trace ve response modelini netlestir.
2. [x] AI response/UI tarafinda kullanilan context'i gorunur yap.
3. [x] Semantic publish/validation icin DB modelini ve akisi tasarla.
4. [x] Relationship/metric validation testlerini artir.
5. [x] Feedback kategorilerini standardize edip AI history ile bagla.
6. [x] AdventureWorks golden eval setini ekle.
7. [x] DSN encryption ve permission strict-mode islerini kapat.

Composite model maddeleri review'a gore **MVP E**'ye otelendi; detaylari P1.5
fazinda durmaya devam ediyor ama bu listede yer almiyor cunku base semantic
model + AI flow + router taş gibi calistiktan sonra ele alinacak.

## Review Notlari (Kasim 2026)

External review raporundan ozet kararlar. Roadmap'in cekirdek mimarisi
(`LogicalQuery → Validate → Compile → Execute` + semantic context + AI
text-to-LogicalQuery) **dogru** kabul edildi. Asagidakiler eksik gorulen veya
yanlis sirada planlanmis maddeler.

### Mimari Karar Dogrulamalari

- [x] AI raw SQL degil, `LogicalQuery` JSON uretir.
- [x] SQL'i backend compiler uretir.
- [x] Semantic layer raw schema degil, context layer.
- [x] Semantic layer uc seviyede:
  - raw metadata (datasource sync)
  - base semantic model (orders, customers, invoices, ...)
  - composite semantic model (sales_performance, customer_360, ...)
- [x] Composite model SQL view degildir; published business context + approved
  join graph + exposed fields.

### Eksik Bulunan ve Eklenmesi Onerilen Maddeler

- [x] **LogicalQuery versioning:** `Version` alani + `CurrentLogicalQueryVersion`
  + `EnsureVersion()` helper; query history ve audit JSONB'sine versiyon
  bilgisi yaziliyor.
- [x] **Time grain support:** `GroupBy.TimeGrain` (day/week/month/quarter/year);
  compiler SELECT+GROUP BY'i dialect DateTrunc/CalendarPart ile sariyor.
  MySQL DateTrunc bug'i (sabit `DATE_FORMAT`) part-aware varyantlara dusurulerek
  duzeltildi. 4 dialect icin golden SQL testi.
- [x] **HAVING support:** Compiler aggregate substitution ile `HAVING SUM(...) > $N`
  uretiyor; validator HAVING alanlarinin metric olmasini ve op listesini
  zorluyor; AI prompt threshold sorularini HAVING'e yonlendiriyor.
- [x] **Clarification response model:** `Clarification{Status, Question, Reason,
  Options, Candidates, Source}` envelope eklendi; router belirsiz kaldiginda
  `ClarificationFromRouting()` aday tablolari `ClarificationOption`/
  `ClarificationContext` olarak doluyor. Legacy string alanlar geri uyumluluk
  icin korundu.
- [x] **Query fingerprint:** `ComputeFingerprint(FingerprintInputs)` =
  SHA-256(canonical LogicalQuery + datasource_id + context_version +
  permission_scope). Filter/GroupBy/Having siralama-bagimsiz; Select/OrderBy
  sirasi anlam tasidigi icin korunuyor. `query_history` ve `ai_query_history`
  tablolarina `query_fingerprint` kolonu + partial index migration'i eklendi.
- [x] **Semantic context size budget:** `ContextBudget` (MaxModels/Dimensions/
  Metrics/Joins/PromptChars) + `DefaultContextBudget()` + `EnforceBudget()`;
  `ValidateContext` publish oncesi ihlalleri warning olarak ekliyor.
- [x] **Chart-ready result metadata:** `ResultColumn.SemanticType` (dimension|
  metric) + `Format` (number|currency|percent|date|datetime|text); `Result`'a
  `ChartSuggestions` (bar|line|table|number|pie). `QueryService.Run` Execute
  sonrasi `EnrichResult` ile semantic eslestirme ve chart kuralina gore oneri
  uretiyor.

### Cekirdek Riskler

- [ ] **Metric expression guvenligi:** Ilk surumde metric expression serbest
  SQL olmasin. Sadece `source_column` + `aggregation`. Calculated metric
  gerekirse kontrollu AST (`divide`, `multiply`, `subtract`, `add`) ile.
  Serbest SQL expression cok gec gelmeli. (Mevcut `semantic_metrics.expression`
  hala SQL fragment kabul ediyor; takip et.)
- [x] **Join fanout:** Planner `many_to_one` guvenli, `one_to_many`/`many_to_many`
  + metric icin warning/reject uretiyor; sessiz yanlis sonuc yok.
- [x] **Permission cross-layer:** Permission semantic context builder, AI prompt
  builder, LogicalQuery validator, SQL compiler ve audit log'da uygulaniyor;
  permission disi field AI prompt'a girmiyor.
- [x] **TableRouter ilk surumde basit kalsin:** Default akis exact/synonym/
  column-keyword/semantic-name/FK-neighbor. Embedding opsiyonel boost olarak
  duruyor; kapaliyken keyword-first calisiyor. Question entity tokenlari icin
  FK expansion da eklendi (`appendQuestionEntityTables`).
- [x] **Multi-database sirasi:** PostgreSQL compiler oturmus, dialect interface
  stabilize, golden SQL testleri 4 dialect icin yesil; MySQL DateTrunc fix
  bu fazda yapildi.

### Composite Model Konusunda Revize

Composite model fikri **mantikli** ve uzun vadede yuksek deger. Fakat
LogicalQuery, validator, planner, prompt builder, TableRouter, permission,
audit, query history, UI ve test katmanlarinin hepsine dokundugu icin **erken**
eklenirse sistemi gereginden fazla karmasiklastirir.

Revize edilmis sira:

1. **Faz Sonu:** Composite model ancak base semantic model + AI flow + router
   taş gibi calistiktan sonra eklensin (zaten dosyada P1.5 fazinda).
2. **Basit version/rollback:** Ilk etapta ayri snapshot tablosu yerine
   `status: draft | published | archived` + `version: int` yeterli. Eski
   published kayitlari `archived` olarak tutmak rollback icin yeterli olabilir.
3. **Composite join referansi:** `join_name` yerine mumkunse base semantic
   join'e `source_join_id` ile referans ver. Composite join'i sifirdan tarif
   etmek yerine base semantic join'in ustune kurulsun.

```sql
semantic_composite_joins (
  id,
  composite_model_id,
  source_join_id,         -- base semantic_joins(id) referansi
  from_model_id,
  to_model_id,
  required,
  risk_level
)
```

### Onerilen Uygulama Sirasi (Revize)

Roadmap'teki MVP listesi cok genis; review'a gore boyle dilimle:

1. **MVP A — Backend Query Engine:** PostgreSQL only, manuel semantic model,
   LogicalQuery API, SQL compiler, query execution, query history, golden
   SQL tests. **[TAMAMLANDI]**
2. **MVP B — AI:** Question → LogicalQuery, prompt context from semantic
   model, preview, run-after-validation, AI history, clarification.
   **[TAMAMLANDI]**
3. **MVP C — Multi-database:** MySQL, SQL Server, ClickHouse + dialect tests +
   golden SQL coverage. **[TAMAMLANDI]** (MySQL DateTrunc fix dahil)
4. **MVP D — Governance:** Permissions, audit, read-only enforcement, row-level
   filters, semantic publish/versioning. **[TAMAMLANDI]**
5. **MVP F — Frontend:** Datasource setup, metadata browser, semantic model
   editor, AI question page, context trace, query result table, simple charts.
   **[TAMAMLANDI]**
6. **MVP E — Composite Model:** Composite semantic model, approved join paths,
   composite prompt context, composite planner support, composite audit.
   **[BEKLEMEDE]** Review composite'i en sona oneriyor; base flow oturmus
   durumda, ihtiyac netlestiginde acilacak.

### Composite Model'i Erken Eklememe Gerekcesi

Composite model su katmanlara dokunur:

- LogicalQuery
- Validator
- Planner
- Prompt builder
- TableRouter
- Permission
- Audit
- Query history
- UI
- Tests

Erken eklemek silent regression riski yaratir. Once base semantic model + query
compiler + AI flow taş gibi calismali.

### En Net Karar

Roadmap'ten en mantikli ana omurga su:

```text
Metadata sync
   ↓
Semantic model
   ↓
LogicalQuery
   ↓
Validation
   ↓
Dialect compiler
   ↓
Safe execution
   ↓
AI -> LogicalQuery
   ↓
Context trace
   ↓
Feedback / eval
   ↓
Composite semantic model
```

Asil guvenlik ve dogruluk uclusu: **Semantic Resolver + LogicalQuery Validator +
SQL Compiler.** Bu uclu saglam olursa Biqly cok iyi bir mimariye oturur.

## Kaynak Referanslar

- https://github.com/Canner/WrenAI
- https://docs.getwren.ai/oss/overview/introduction
- https://docs.getwren.ai/oss/engine/concept/what_is_mdl
- https://docs.getwren.ai/oss/engine/guide/modeling/overview
- https://docs.getwren.ai/oss/engine/guide/modeling/model
- https://docs.getwren.ai/oss/engine/guide/modeling/relation
