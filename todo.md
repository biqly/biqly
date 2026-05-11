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

1. `TableRouter` debug trace ve response modelini netlestir.
2. AI response/UI tarafinda kullanilan context'i gorunur yap.
3. Semantic publish/validation icin DB modelini ve akisi tasarla.
4. Relationship/metric validation testlerini artir.
5. Feedback kategorilerini standardize edip AI history ile bagla.
6. AdventureWorks golden eval setini ekle.
7. DSN encryption ve permission strict-mode islerini kapat.

## Kaynak Referanslar

- https://github.com/Canner/WrenAI
- https://docs.getwren.ai/oss/overview/introduction
- https://docs.getwren.ai/oss/engine/concept/what_is_mdl
- https://docs.getwren.ai/oss/engine/guide/modeling/overview
- https://docs.getwren.ai/oss/engine/guide/modeling/model
- https://docs.getwren.ai/oss/engine/guide/modeling/relation
