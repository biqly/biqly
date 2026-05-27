import type { JSX } from 'react'

export function TermsOfUseEN(): JSX.Element {
  return (
    <div style={{ lineHeight: '1.6', color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
      <h3 style={{ color: 'var(--text)', marginTop: '0' }}>1. Acceptance of Terms</h3>
      <p>
        By creating an account, connecting a database, or using the ABI platform ("Service"), you agree to be bound by these Terms of Use. If you do not agree to these terms, do not use the Service.
      </p>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>2. Description of Service</h3>
      <p>
        ABI is an AI-powered Business Intelligence (BI) query engine. It translates natural language questions into LogicalQuery structures, which are then compiled into parameterized SQL queries executed against your configured databases.
      </p>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>3. AI Generation & Database Safety</h3>
      <ul style={{ paddingLeft: '1.5rem', margin: '0.75rem 0' }}>
        <li style={{ marginBottom: '0.5rem' }}><strong>AI Output Limitations:</strong> You acknowledge that ABI uses Large Language Models (LLMs) to generate queries. AI-generated queries can occasionally be inaccurate, incomplete, or contain hallucinations. You are responsible for reviewing and verifying queries before relying on them.</li>
        <li style={{ marginBottom: '0.5rem' }}><strong>ReadOnly & Timeout Safety:</strong> To protect your database infrastructure, ABI automatically enforces strict security limits: read-only connection verification, maximum execution timeouts (default 30 seconds), and maximum row limits (default 10,000 rows). You must not attempt to bypass these safety constraints.</li>
        <li style={{ marginBottom: '0.5rem' }}><strong>Security Rules:</strong> Direct modification commands (e.g., INSERT, UPDATE, DELETE, DROP, ALTER) are strictly rejected by the semantic compiler and the read-only checker.</li>
      </ul>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>4. Credentials & Access</h3>
      <p>
        You are responsible for safeguarding your login credentials (including Passkeys and MFA recovery codes) and for all activities that occur under your account. You agree to use read-only database credentials when configuring connection strings (DSNs) in the Service.
      </p>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>5. Limitation of Liability</h3>
      <p>
        To the maximum extent permitted by law, ABI shall not be liable for any indirect, incidental, special, consequential, or punitive damages, including but not limited to loss of profits, data loss, database downtime, performance degradation, or security breaches resulting from your use of the Service.
      </p>
    </div>
  )
}

export function TermsOfUseTR(): JSX.Element {
  return (
    <div style={{ lineHeight: '1.6', color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
      <h3 style={{ color: 'var(--text)', marginTop: '0' }}>1. Koşulların Kabulü</h3>
      <p>
        Bir hesap oluşturarak, bir veri tabanı bağlayarak veya ABI platformunu ("Hizmet") kullanarak, bu Kullanım Koşullarına bağlı kalmayı kabul etmiş olursunuz. Bu koşulları kabul etmiyorsanız, Hizmet'i kullanmayın.
      </p>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>2. Hizmet Tanımı</h3>
      <p>
        ABI, yapay zeka destekli bir İş Zekası (BI) sorgu motorudur. Doğal dildeki soruları Mantıksal Sorgu (LogicalQuery) yapılarına dönüştürür ve bu yapılar daha sonra yapılandırılmış veri tabanlarınızda çalıştırılmak üzere parametrik SQL sorgularına derlenir.
      </p>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>3. Yapay Zeka Üretimi ve Veri Tabanı Güvenliği</h3>
      <ul style={{ paddingLeft: '1.5rem', margin: '0.75rem 0' }}>
        <li style={{ marginBottom: '0.5rem' }}><strong>Yapay Zeka Sınırlandırmaları:</strong> ABI'nin sorgu üretmek için Büyük Dil Modelleri (LLM) kullandığını kabul etmektesiniz. Yapay zeka tarafından üretilen sorgular zaman zaman hatalı, eksik veya yanıltıcı olabilir. Sorguları temel almadan önce gözden geçirmek ve doğrulamak sizin sorumluluğunuzdadır.</li>
        <li style={{ marginBottom: '0.5rem' }}><strong>Salt Okunur ve Zaman Aşımı Güvenliği:</strong> Veri tabanı altyapınızı korumak için ABI otomatik olarak katı güvenlik limitleri uygular: salt okunur bağlantı doğrulaması, maksimum çalışma zaman aşımı (varsayılan 30 saniye) ve maksimum satır limitleri (varsayılan 10.000 satır). Bu güvenlik sınırlarını aşmaya çalışmamalısınız.</li>
        <li style={{ marginBottom: '0.5rem' }}><strong>Güvenlik Kuralları:</strong> Doğrudan veri değiştirme komutları (örn. INSERT, UPDATE, DELETE, DROP, ALTER) anlamsal derleyici ve salt okunur denetleyicisi tarafından kesin olarak reddedilir.</li>
      </ul>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>4. Kimlik Bilgileri ve Erişim</h3>
      <p>
        Giriş bilgilerinizi (Passkey'ler ve MFA kurtarma kodları dahil) korumaktan ve hesabınız altında gerçekleşen tüm faaliyetlerden siz sorumlusunuz. Hizmet üzerinde bağlantı dizesi (DSN) yapılandırırken salt okunur veri tabanı kimlik bilgileri kullanmayı kabul edersiniz.
      </p>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>5. Sorumluluk Sınırlandırması</h3>
      <p>
        Yasaların izin verdiği azami ölçüde ABI; kâr kaybıı, veri kaybı, veri tabanı kesintisi, performans düşüşü veya Hizmet kullanımınızdan kaynaklanan güvenlik ihlalleri dahil ancak bunlarla sınırlı olmamak üzere hiçbir dolaylı, arızi, özel veya cezai zarardan sorumlu tutulamaz.
      </p>
    </div>
  )
}

export function PrivacyPolicyEN(): JSX.Element {
  return (
    <div style={{ lineHeight: '1.6', color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
      <h3 style={{ color: 'var(--text)', marginTop: '0' }}>1. Information We Collect</h3>
      <ul style={{ paddingLeft: '1.5rem', margin: '0.75rem 0' }}>
        <li style={{ marginBottom: '0.5rem' }}><strong>Account Information:</strong> We collect your name, email address, and authentication credentials (such as Passkey public keys and MFA metadata) when you register.</li>
        <li style={{ marginBottom: '0.5rem' }}><strong>Metadata & Schemas:</strong> To build accurate context for AI query generation, we introspect and store database schema metadata (table names, column types, foreign keys, and descriptions).</li>
        <li style={{ marginBottom: '0.5rem' }}><strong>Audit & Query History:</strong> We log query logs containing execution status, performance metrics, duration, query fingerprints, and generated SQL.</li>
      </ul>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem', padding: '0.75rem', borderLeft: '4px solid var(--accent)', backgroundColor: 'rgba(255, 255, 255, 0.02)', borderRadius: '0 0.5rem 0.5rem 0' }}>
        <strong style={{ color: 'var(--text)' }}>2. Zero Data Exfiltration Guarantee (Crucial)</strong>
        <p style={{ margin: '0.25rem 0 0 0', fontSize: '0.85rem' }}>
          ABI does <strong>not</strong> send your database's actual data rows or query results to LLM providers (e.g., OpenAI or Anthropic). We only transmit schema names, synonyms, descriptions, and the text of your natural language question to the AI. The resulting LogicalQuery is compiled locally, and raw database results are returned directly to your browser without leaving our secure environment.
        </p>
      </h3>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>3. Data Security</h3>
      <p>
        All Database Connection Strings (DSNs) are stored in our metadata database in a highly secure, encrypted format using AES-256 with a unique encryption key. Access to connection credentials is strictly audited and limited to secure query execution pipelines.
      </p>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>4. AI Providers</h3>
      <p>
        We share natural language questions and schema metadata with third-party LLM providers (OpenAI, Anthropic) as configured by your environment. We configure these integrations to ensure your inputs are not used for training public models.
      </p>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>5. Your Rights</h3>
      <p>
        Depending on your location (e.g., GDPR), you have the right to access, rectify, or delete your personal data, as well as restrict its processing. To exercise these rights, please contact your workspace administrator.
      </p>
    </div>
  )
}

export function PrivacyPolicyTR(): JSX.Element {
  return (
    <div style={{ lineHeight: '1.6', color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
      <h3 style={{ color: 'var(--text)', marginTop: '0' }}>1. Topladığımız Bilgiler</h3>
      <ul style={{ paddingLeft: '1.5rem', margin: '0.75rem 0' }}>
        <li style={{ marginBottom: '0.5rem' }}><strong>Hesap Bilgileri:</strong> Kayıt olduğunuzda adınızı, e-posta adresinizi ve kimlik doğrulama bilgilerini (Passkey açık anahtarları ve MFA meta verileri gibi) toplarız.</li>
        <li style={{ marginBottom: '0.5rem' }}><strong>Meta Veri ve Şemalar:</strong> Yapay zeka sorgu üretimi için doğru bağlam oluşturmak amacıyla, veri tabanı şeması meta verilerini (tablo adları, sütun tipleri, yabancı anahtarlar ve açıklamalar) inceler ve depolarız.</li>
        <li style={{ marginBottom: '0.5rem' }}><strong>Denetim ve Sorgu Geçmişi:</strong> Çalışma durumu, performans ölçümleri, süre, sorgu parmak izleri ve üretilen SQL sorgularını içeren denetim günlüklerini kaydederiz.</li>
      </ul>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem', padding: '0.75rem', borderLeft: '4px solid var(--accent)', backgroundColor: 'rgba(255, 255, 255, 0.02)', borderRadius: '0 0.5rem 0.5rem 0' }}>
        <strong style={{ color: 'var(--text)' }}>2. Sıfır Veri Sızıntısı Garantisi (Kritik)</strong>
        <p style={{ margin: '0.25rem 0 0 0', fontSize: '0.85rem' }}>
          ABI, veri tabanınızın gerçek veri satırlarını veya sorgu sonuçlarını <strong>asla</strong> üçüncü taraf yapay zeka sağlayıcılarına (örn. OpenAI veya Anthropic) göndermez. Yapay zekaya yalnızca şema adları, eş anlamlılar, açıklamalar ve doğal dildeki sorunuz iletilir. Üretilen mantıksal sorgu yerel olarak derlenir ve veri tabanı sonuçları güvenli ortamımızdan dışarı çıkmadan doğrudan tarayıcınıza iletilir.
        </p>
      </h3>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>3. Veri Güvenliği</h3>
      <p>
        Tüm Veri Tabanı Bağlantı Dizeleri (DSN'ler), benzersiz bir şifreleme anahtarı kullanılarak AES-256 ile yüksek güvenlikli şifrelenmiş bir biçimde meta veri veri tabanımızda depolanır. Bağlantı bilgilerine erişim sıkı bir şekilde denetlenir ve yalnızca güvenli sorgu yürütme iş akışlarıyla sınırlıdır.
      </p>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>4. Yapay Zeka Sağlayıcıları</h3>
      <p>
        Doğal dil sorularını ve şema meta verilerini, ortamınız tarafından yapılandırılan üçüncü taraf yapay zeka sağlayıcılarıyla (OpenAI, Anthropic) paylaşırız. Bu entegrasyonları, girdilerinizin genel modelleri eğitmek için kullanılmamasını sağlayacak şekilde yapılandırırız.
      </p>

      <h3 style={{ color: 'var(--text)', marginTop: '1.5rem' }}>5. Haklarınız</h3>
      <p>
        Bulunduğunuz yere bağlı olarak (örn. KVKK/GDPR), kişisel verilerinize erişme, bunları düzeltme veya silme ve işlenmesini kısıtlama hakkına sahipsiniz. Bu haklarınızı kullanmak için lütfen çalışma alanı yöneticinizle iletişime geçin.
      </p>
    </div>
  )
}
