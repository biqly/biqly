// Starter knowledge files scaffolded on demand — the "default skills" a team
// needs on day one: where data lives, how core calculations work, canonical
// terms and a worked example. All content is reviewable markdown the user
// edits and publishes like any other file.

export interface StarterFile {
  path: string
  content: string
}

export const STARTER_FILES: StarterFile[] = [
  {
    path: 'README.md',
    content: `---
title: "Bilgi tabanı"
description: "Bu klasör AI'ya verinizi, iş kurallarınızı ve terimlerinizi öğretir."
---

# Bilgi tabanı

Bu dosyalar AI'nın soruları doğru yanıtlaması için projeye özgü bağlam sağlar.
Her şey markdown — tarayıcıda düzenleyin, hazır olunca **Yayınla**'ya basın.

## Klasörler

- **glossary/** — iş terimleri ve eş anlamlıları
- **instructions/** — SQL üretirken uyulacak kurallar ("X buradadır", "Y hariç tutulur")
- **metrics/** — metrik tanımları ve hesaplama adımları
- **sql-pairs/** — soru → SQL örnek çözümleri

> Bir dosyayı yayınladığınızda ilgili yapısal kayıtlar otomatik çıkarılır;
> agent'lar da dosyayı doğrudan okuyabilir.
`,
  },
  {
    path: 'instructions/veri-konumlari.md',
    content: `---
type: instruction
title: "Veri konumları"
description: "Hangi veri hangi tabloda/kolonda bulunur — soru yanıtlarken önce buraya bak."
---

# Veri konumları

Aşağıya "bu veri buradadır" satırlarını ekleyin; AI sorgu üretirken bu
eşlemeleri kullanır.

| Veri | Nerede | Not |
| --- | --- | --- |
| örn. Müşteri e-postası | \`customers.email\` | PII — maskeli döner |
| örn. Sipariş tutarı | \`orders.amount\` | KDV dahil |

## Usage notes

Bir soru "nerede/hangi tablo" belirsizliği taşıyorsa önce bu dosyadaki
eşlemeler geçerlidir.
`,
  },
  {
    path: 'metrics/ornek-metrik.md',
    content: `---
type: metric
title: "Örnek metrik: Net ciro"
description: "Net ciro hesabının tanımı ve adımları — kendi metriğinizle değiştirin."
---

# Net ciro

**Tanım:** İadeler düşüldükten sonraki toplam satış tutarı.

**Birim:** TRY · **Kırılım:** günlük

## Hesaplama

1. \`orders.amount\` toplanır (\`status = 'completed'\`)
2. \`refunds.amount\` toplamı düşülür
3. Sonuç güne göre gruplanır

## Usage notes

"Ciro" tek başına geçtiğinde bu net tanım kullanılır; brüt istenirse
soruda açıkça "brüt" denmelidir.
`,
  },
  {
    path: 'glossary/ornek-terim.md',
    content: `---
type: glossary
term: "aktif kullanıcı"
aliases: ["active user", "aktif üye"]
description: "Son 30 günde en az bir oturum açmış kullanıcı."
---

# aktif kullanıcı

Son 30 günde en az bir oturum açmış kullanıcı.

## Usage notes

"Kaç kullanıcı var" gibi sorularda aksi belirtilmedikçe bu tanım geçerlidir.
`,
  },
  {
    path: 'sql-pairs/ornek-soru.md',
    content: `---
type: sql-pair
question: "geçen ay kaç sipariş verildi?"
description: "Aylık sipariş sayısı için örnek çözüm — kendi örneğinizle değiştirin."
---

# Geçen ay kaç sipariş verildi?

\`\`\`sql
SELECT count(*) AS siparis_sayisi
FROM orders
WHERE created_at >= date_trunc('month', now()) - interval '1 month'
  AND created_at < date_trunc('month', now())
\`\`\`
`,
  },
]
