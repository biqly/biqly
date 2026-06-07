# -*- coding: utf-8 -*-
"""
Biqly — Teknik Mimari & Best-Practice Analiz Dokümanı (Türkçe) PDF üreteci.
reportlab ile, gömülü vektörel diyagramlar ve Türkçe-uyumlu fontlarla.
"""
import os
from reportlab.lib.pagesizes import A4
from reportlab.lib.units import mm
from reportlab.lib import colors
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.enums import TA_LEFT, TA_CENTER, TA_JUSTIFY
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import (
    BaseDocTemplate, PageTemplate, Frame, Paragraph, Spacer, Table, TableStyle,
    PageBreak, KeepTogether, Flowable, ListFlowable, ListItem,
)
from reportlab.platypus.tableofcontents import TableOfContents
from reportlab.graphics.shapes import Drawing, Rect, String, Line, Polygon, PolyLine

OUT = os.path.join(os.path.dirname(__file__), "biqly_analiz.pdf")

# --------------------------------------------------------------------------
# Fontlar (Türkçe ğ/ş/ı/İ desteği için TTF)
# --------------------------------------------------------------------------
FONTS = {
    "Body": "/System/Library/Fonts/Supplemental/Arial.ttf",
    "Body-Bold": "/System/Library/Fonts/Supplemental/Arial Bold.ttf",
    "Body-Italic": "/System/Library/Fonts/Supplemental/Arial Italic.ttf",
    "Body-BoldItalic": "/System/Library/Fonts/Supplemental/Arial Bold Italic.ttf",
    "Mono": "/System/Library/Fonts/Supplemental/Courier New.ttf",
    "Mono-Bold": "/System/Library/Fonts/Supplemental/Courier New Bold.ttf",
}
for name, path in FONTS.items():
    if os.path.exists(path):
        pdfmetrics.registerFont(TTFont(name, path))
# aileleri bağla
pdfmetrics.registerFontFamily(
    "Body", normal="Body", bold="Body-Bold",
    italic="Body-Italic" if os.path.exists(FONTS["Body-Italic"]) else "Body",
    boldItalic="Body-BoldItalic" if os.path.exists(FONTS["Body-BoldItalic"]) else "Body-Bold",
)
pdfmetrics.registerFontFamily("Mono", normal="Mono", bold="Mono-Bold",
                              italic="Mono", boldItalic="Mono-Bold")

# --------------------------------------------------------------------------
# Renk paleti
# --------------------------------------------------------------------------
NAVY    = colors.HexColor("#1F2A44")
BLUE    = colors.HexColor("#2D6CDF")
BLUE_L  = colors.HexColor("#E8F0FE")
GREEN   = colors.HexColor("#0E9F6E")
GREEN_L = colors.HexColor("#E3F6EF")
AMBER   = colors.HexColor("#D97706")
AMBER_L = colors.HexColor("#FDF1DF")
RED     = colors.HexColor("#DC2626")
RED_L   = colors.HexColor("#FBE7E7")
SLATE   = colors.HexColor("#475569")
SLATE_L = colors.HexColor("#F1F5F9")
PURPLE  = colors.HexColor("#7C3AED")
PURPLE_L= colors.HexColor("#F0EAFB")
TEAL    = colors.HexColor("#0D9488")
TEAL_L  = colors.HexColor("#E0F2F1")
GREY_LN = colors.HexColor("#CBD5E1")
INK     = colors.HexColor("#0F172A")

def hx(c):
    return "#" + c.hexval()[2:]

PAGE_W, PAGE_H = A4
ML, MR, MT, MB = 20*mm, 18*mm, 22*mm, 20*mm
CONTENT_W = PAGE_W - ML - MR

# --------------------------------------------------------------------------
# Stiller
# --------------------------------------------------------------------------
ss = getSampleStyleSheet()

def S(name, **kw):
    base = kw.pop("parent", None)
    return ParagraphStyle(name, parent=base, **kw)

st_title   = S("t", fontName="Body-Bold", fontSize=30, leading=36, textColor=NAVY)
st_subtitle= S("st", fontName="Body", fontSize=14, leading=20, textColor=SLATE)
st_cover_meta = S("cm", fontName="Body", fontSize=10.5, leading=16, textColor=SLATE)

st_h1 = S("h1", fontName="Body-Bold", fontSize=19, leading=24, textColor=NAVY,
          spaceBefore=6, spaceAfter=10)
st_h2 = S("h2", fontName="Body-Bold", fontSize=14, leading=18, textColor=BLUE,
          spaceBefore=14, spaceAfter=6)
st_h3 = S("h3", fontName="Body-Bold", fontSize=11.5, leading=15, textColor=NAVY,
          spaceBefore=10, spaceAfter=4)
st_body = S("body", fontName="Body", fontSize=10, leading=15.5, textColor=INK,
            alignment=TA_JUSTIFY, spaceAfter=6)
st_bul  = S("bul", parent=st_body, alignment=TA_LEFT, spaceAfter=3, leading=14.5)
st_cap  = S("cap", fontName="Body-Italic", fontSize=8.6, leading=12, textColor=SLATE,
            alignment=TA_CENTER, spaceBefore=4, spaceAfter=12)
st_code = S("code", fontName="Mono", fontSize=8.2, leading=11.5, textColor=INK,
            backColor=SLATE_L, borderPadding=(6,6,6,6), spaceBefore=2, spaceAfter=8)
st_tbl  = S("tbl", fontName="Body", fontSize=8.6, leading=11.5, textColor=INK)
st_tblb = S("tblb", fontName="Body-Bold", fontSize=8.6, leading=11.5, textColor=colors.white)
st_tblbd= S("tblbd", fontName="Body-Bold", fontSize=8.6, leading=11.5, textColor=INK)
st_toc1 = S("toc1", fontName="Body-Bold", fontSize=11, leading=20, textColor=NAVY)
st_toc2 = S("toc2", fontName="Body", fontSize=9.5, leading=16, textColor=SLATE, leftIndent=14)
st_quote= S("quote", fontName="Body-Italic", fontSize=9.5, leading=14, textColor=SLATE,
            leftIndent=10, spaceAfter=8)

# --------------------------------------------------------------------------
# Yardımcılar: başlık (TOC + bookmark)
# --------------------------------------------------------------------------
_seq = [0]
class Heading(Paragraph):
    def __init__(self, text, style, level):
        self.level = level
        _seq[0] += 1
        self.bm_key = "h%d" % _seq[0]
        Paragraph.__init__(self, text, style)
    def draw(self):
        Paragraph.draw(self)
        if self.level <= 1:
            self.canv.bookmarkPage(self.bm_key)
            self.canv.addOutlineEntry(self.getPlainText(), self.bm_key, self.level, False)

def H1(t): return Heading(t, st_h1, 0)
def H2(t): return Heading(t, st_h2, 1)
def H3(t): return Heading(t, st_h3, 2)
def P(t):  return Paragraph(t, st_body)
def Q(t):  return Paragraph(t, st_quote)
def CODE(t):
    t = t.replace("&","&amp;").replace("<","&lt;").replace(">","&gt;").replace(" ","&nbsp;").replace("\n","<br/>")
    return Paragraph(t, st_code)
def CAP(t): return Paragraph(t, st_cap)
def SP(h=6): return Spacer(1, h)

def BUL(items):
    li = []
    for it in items:
        li.append(ListItem(Paragraph(it, st_bul), leftIndent=12, value="•",
                           bulletColor=BLUE))
    return ListFlowable(li, bulletType="bullet", start="•", bulletColor=BLUE,
                        leftIndent=14, bulletFontName="Body")

def chip(txt, fill, tc=colors.white):
    return Paragraph('<font color="%s"><b>%s</b></font>' % (tc.hexval()[2:] if hasattr(tc,'hexval') else "FFFFFF", txt), st_tbl)

# --------------------------------------------------------------------------
# Tablo yardımcısı
# --------------------------------------------------------------------------
def mk_table(header, rows, col_w, header_bg=NAVY, font=8.6, align_left_cols=None):
    align_left_cols = align_left_cols or list(range(len(header)))
    data = [[Paragraph(h, st_tblb) for h in header]]
    for r in rows:
        row = []
        for c in r:
            row.append(Paragraph(str(c), st_tbl))
        data.append(row)
    t = Table(data, colWidths=col_w, repeatRows=1)
    style = [
        ("BACKGROUND",(0,0),(-1,0),header_bg),
        ("VALIGN",(0,0),(-1,-1),"MIDDLE"),
        ("TOPPADDING",(0,0),(-1,-1),4),
        ("BOTTOMPADDING",(0,0),(-1,-1),4),
        ("LEFTPADDING",(0,0),(-1,-1),5),
        ("RIGHTPADDING",(0,0),(-1,-1),5),
        ("LINEBELOW",(0,0),(-1,-1),0.4,GREY_LN),
        ("LINEAFTER",(0,0),(-2,-1),0.3,colors.HexColor("#E2E8F0")),
        ("ROWBACKGROUNDS",(0,1),(-1,-1),[colors.white, SLATE_L]),
    ]
    t.setStyle(TableStyle(style))
    return t

# --------------------------------------------------------------------------
# DİYAGRAM ARAÇ SETİ
# --------------------------------------------------------------------------
def _wrap_lines(text):
    return text.split("\n")

def box(d, x, y, w, h, text, fill, stroke=None, tcolor=colors.white,
        fs=8.5, rx=5, bold=True):
    stroke = stroke or fill
    d.add(Rect(x, y, w, h, rx=rx, ry=rx, fillColor=fill, strokeColor=stroke,
               strokeWidth=1))
    lines = _wrap_lines(text)
    fn = "Body-Bold" if bold else "Body"
    total = len(lines)*(fs+2)
    cy = y + h/2 + total/2 - fs
    for ln in lines:
        d.add(String(x+w/2, cy, ln, fontName=fn, fontSize=fs, fillColor=tcolor,
                     textAnchor="middle"))
        cy -= (fs+2)
    return (x, y, w, h)

def arrow(d, x1, y1, x2, y2, color=SLATE, w=1.1, label=None, dashed=False,
          lfs=7, lcolor=None):
    line = Line(x1, y1, x2, y2, strokeColor=color, strokeWidth=w)
    if dashed:
        line.strokeDashArray = [3,2]
    d.add(line)
    # ok başı
    import math
    ang = math.atan2(y2-y1, x2-x1)
    sz = 5
    p1 = (x2, y2)
    p2 = (x2 - sz*math.cos(ang - math.pi/7), y2 - sz*math.sin(ang - math.pi/7))
    p3 = (x2 - sz*math.cos(ang + math.pi/7), y2 - sz*math.sin(ang + math.pi/7))
    d.add(Polygon([p1[0],p1[1],p2[0],p2[1],p3[0],p3[1]], fillColor=color, strokeColor=color))
    if label:
        mx, my = (x1+x2)/2, (y1+y2)/2
        d.add(String(mx, my+3, label, fontName="Body", fontSize=lfs,
                     fillColor=lcolor or SLATE, textAnchor="middle"))

def title_band(d, w, y, text, color=NAVY):
    d.add(String(w/2, y, text, fontName="Body-Bold", fontSize=9.5,
                 fillColor=color, textAnchor="middle"))

# ---- Diyagram 1: Servis / Mimari topolojisi (ortogonal) -------------------
def _vseg(d, x, y1, y2, color, w=0.9, dashed=False):
    ln = Line(x, y1, x, y2, strokeColor=color, strokeWidth=w)
    if dashed:
        ln.strokeDashArray = [3, 2]
    d.add(ln)

def _hseg(d, x1, x2, y, color, w=0.9, dashed=False):
    ln = Line(x1, y, x2, y, strokeColor=color, strokeWidth=w)
    if dashed:
        ln.strokeDashArray = [3, 2]
    d.add(ln)

def diagram_architecture():
    W, H = CONTENT_W, 460
    d = Drawing(W, H)
    cx = W/2
    gap = 9

    # --- Üst katman ---
    box(d, cx-78, H-32, 156, 28, "İstemci (Tarayıcı / SPA)", NAVY, fs=8.5)
    box(d, cx-150, H-104, 300, 36,
        "Gateway «lan-gw» · Gateway API (HTTPRoute)\nhost: abi.il1.nl · PathPrefix ile doğrudan yönlendirme",
        PURPLE, fs=7.6)
    arrow(d, cx, H-32, cx, H-68, SLATE, w=1.2, label="HTTPS", lfs=7)

    # --- Mikroservis satırı (prod: her biri ayrı Deployment) ---
    sy = H-225
    svcs = [("frontend\n:8080", TEAL, "/"),
            ("catalog\n:8080", GREEN, "/api/datasources·\nmetadata·semantic"),
            ("query\n:8081", GREEN, "/api/query"),
            ("ai\n:8082", GREEN, "/api/ai"),
            ("auth\n:8889", AMBER, "/api/auth"),
            ("mail\n:8890", AMBER, "iç servis")]
    n = len(svcs)
    pw = (W - (n-1)*gap)/n
    xs = []
    for i, (t, c, _) in enumerate(svcs):
        x = i*(pw+gap)
        box(d, x, sy, pw, 36, t, c, fs=7.4)
        xs.append(x+pw/2)
    svc_top, svc_bot = sy+36, sy

    # Gateway -> ilk 5 servise ortogonal bus (mail iç servis)
    grail = H-150
    exposed = xs[:5]
    _vseg(d, cx, H-104, grail, SLATE, 1.2)
    _hseg(d, min(exposed), max(exposed), grail, SLATE, 1.0)
    for i, xc in enumerate(exposed):
        arrow(d, xc, grail, xc, svc_top, SLATE, 0.9)
        d.add(String(xc, svc_top+4, svcs[i][2].split("\n")[0], fontName="Body",
                     fontSize=5.8, fillColor=SLATE, textAnchor="middle"))
    # auth -> mail (iç çağrı)
    arrow(d, xs[4], sy+18, xs[5], sy+18, AMBER, 0.9, dashed=True)

    # --- Alt sıra: Harici LLM + 3 veri deposu ---
    dy = 34
    bottom = [("Harici LLM\n(OpenAI uyumlu)", PURPLE),
              ("PostgreSQL :5432\n(StatefulSet · 3 DB)", TEAL),
              ("Dragonfly :6379\n(Redis önbellek)", RED),
              ("NATS :4222\n(JetStream)", PURPLE)]
    bw4 = (W - 3*gap)/4
    bxs = []
    for i, (t, c) in enumerate(bottom):
        x = i*(bw4+gap)
        box(d, x, dy, bw4, 38, t, c, fs=7.0)
        bxs.append(x+bw4/2)
    ds_top = dy+38
    llm_cx, pg_cx, df_cx, nats_cx = bxs

    # backend servisleri -> veri katmanı (ortogonal bus)
    drail = 96
    data_peers = [xs[1], xs[2], xs[4]]  # catalog, query, auth
    for xc in data_peers:
        _vseg(d, xc, svc_bot, drail, TEAL, 0.8)
    _hseg(d, min(data_peers+[pg_cx, df_cx]), max(data_peers+[pg_cx, df_cx]), drail, TEAL, 0.9)
    arrow(d, pg_cx, drail, pg_cx, ds_top, TEAL, 1.0)
    arrow(d, df_cx, drail, df_cx, ds_top, TEAL, 1.0)

    # ai -> Harici LLM (kesik) ve ai -> NATS (içinde tüketici)
    arrow(d, xs[3], svc_bot, llm_cx, ds_top, PURPLE, 1.0, dashed=True, label="LLM", lfs=6.5)
    arrow(d, xs[3], svc_bot, nats_cx, ds_top, PURPLE, 0.9, label="JetStream (ai-içi tüketici)", lfs=6)

    # açıklama
    d.add(String(0, 16, "Prod: 6 bağımsız mikroservis Deployment’ı + Postgres StatefulSet + Dragonfly + NATS. «api» BFF binary "
                 "yalnızca local/docker-compose; ayrı worker pod’u yok — NATS tüketicisi ai pod’u içinde çalışır.",
                 fontName="Body-Italic", fontSize=6.4, fillColor=SLATE))
    d.add(String(0, 6, "Servisler arası: ai→catalog/query (tipli HTTP istemci), tüm servisler→auth (JWT açık-anahtar + izin), "
                 "auth→mail.   - - - kesik: dış LLM / iç asenkron çağrı.",
                 fontName="Body-Italic", fontSize=6.4, fillColor=SLATE))
    return d

# ---- Genel dikey akış diyagramı -------------------------------------------
def _wrap(text, font, fs, maxw):
    words = text.split()
    lines, cur = [], ""
    for w in words:
        t = (cur + " " + w).strip()
        if not cur or pdfmetrics.stringWidth(t, font, fs) <= maxw:
            cur = t
        else:
            lines.append(cur); cur = w
    if cur:
        lines.append(cur)
    return lines

def vertical_flow(steps, accent=BLUE, box_h=None, gap=12):
    """Tam genişlikte, sola hizalı, otomatik satır-kaydıran dikey akış."""
    W = CONTENT_W
    bx, bw, pad, fs, lh = 8, CONTENT_W-16, 9, 8.5, 11.5
    fn = "Body-Bold"
    wrapped = []
    for i, s in enumerate(steps):
        txt = ("%d.  " % (i+1)) + s.replace("\n", " ")
        wrapped.append(_wrap(txt, fn, fs, bw-2*pad))
    heights = [len(ls)*lh + 11 for ls in wrapped]
    H = sum(heights) + gap*(len(steps)-1) + 8
    d = Drawing(W, H)
    y = H - 4
    for i, lines in enumerate(wrapped):
        bh = heights[i]; y -= bh
        col = accent if i % 2 == 0 else NAVY
        d.add(Rect(bx, y, bw, bh, rx=5, ry=5, fillColor=col, strokeColor=col))
        ty = y + bh - pad - fs + 3
        for ln in lines:
            d.add(String(bx+pad, ty, ln, fontName=fn, fontSize=fs, fillColor=colors.white))
            ty -= lh
        if i < len(steps)-1:
            arrow(d, bx+bw/2, y, bx+bw/2, y-gap+1, SLATE, w=1.5)
        y -= gap
    return d

# ---- CI/CD pipeline diyagramı (dikey, ortogonal, kompakt) -----------------
def diagram_cicd():
    W = CONTENT_W
    cx = W/2
    gap = 9
    h_git, h_job, h_st, ag = 26, 46, 30, 19
    stack = [("ghcr.io/biqly/*   ·   imaj etiketi : sha-<commit>", NAVY, "push"),
             ("argocd-image-updater   ·   «chore(deploy): bump image tags» → values-prod.yaml (git)", AMBER, "newest-build algılar"),
             ("ArgoCD auto-sync (prune · selfHeal · ServerSideApply) → namespace «biqly»", GREEN, "git → otomatik senkron")]
    docker_label = ("Docker derle & push   ·   docker-api / docker-frontend + "
                    "build-{ai·query·catalog·auth·mail·migrate}   ·   8 imaj · linux/arm64")
    H = h_git + ag + h_job + ag + h_st + (ag+h_st)*len(stack) + 8
    d = Drawing(W, H)

    # 1) tetikleyici
    gy = H - h_git
    box(d, cx-85, gy, 170, h_git, "git push → main", NAVY, fs=8.5)

    # 2) 3 paralel CI işi
    jy = gy - ag - h_job
    jobs = [("backend\nvet · test -race\nAI eval · build", GREEN),
            ("lint\ngolangci-lint\nv2.12.2", BLUE),
            ("frontend\nnpm run check\n(eslint·knip·test·build)", TEAL)]
    jw = (W - 2*gap)/3
    jxs = []
    for i, (t, c) in enumerate(jobs):
        x = i*(jw+gap)
        box(d, x, jy, jw, h_job, t, c, fs=7.1)
        jxs.append(x+jw/2)
    rail1 = gy - ag/2
    _vseg(d, cx, gy, rail1, SLATE, 1.2)
    _hseg(d, min(jxs), max(jxs), rail1, SLATE, 1.0)
    for xc in jxs:
        arrow(d, xc, rail1, xc, jy+h_job, SLATE, 1.0)
    d.add(String(cx, rail1+3, "paralel · ci.yml + test.yml + semgrep.yml", fontName="Body-Italic",
                 fontSize=6.5, fillColor=SLATE, textAnchor="middle"))

    # 3) Docker derle & push (tam genişlik)
    dy = jy - ag - h_st
    box(d, 0, dy, W, h_st, docker_label, PURPLE, fs=7.1)
    rail2 = jy - ag/2
    for xc in jxs:
        _vseg(d, xc, jy, rail2, SLATE, 0.8)
    _hseg(d, min(jxs), max(jxs), rail2, SLATE, 0.9)
    arrow(d, cx, rail2, cx, dy+h_st, SLATE, 1.0)

    # 4-6) yığılmış tam-genişlik aşamalar
    prev = dy
    for label, color, arlabel in stack:
        sy = prev - ag - h_st
        box(d, 0, sy, W, h_st, label, color, fs=7.2)
        arrow(d, cx, prev, cx, sy+h_st, SLATE, 1.0, label=arlabel, lfs=7)
        prev = sy
    return d

# ---- Deployment / K8s runtime topolojisi (ortogonal) ----------------------
def diagram_k8s():
    W, H = CONTENT_W, 360
    d = Drawing(W, H)
    cx = W/2
    gap = 9
    pad = 14  # namespace iç boşluğu

    # gateway (ns dışında, üstte)
    box(d, cx-100, H-30, 200, 26, "Gateway «lan-gw»  (ns: gateway)", PURPLE, fs=7.8)

    # namespace çerçevesi
    ns_top = H-44
    d.add(Rect(2, 6, W-4, ns_top-6, rx=8, ry=8, fillColor=colors.HexColor("#FAFBFE"),
               strokeColor=BLUE, strokeWidth=1.2, strokeDashArray=[4, 3]))
    d.add(String(12, ns_top-16, "Kubernetes namespace «biqly»   ·   Pod Security: restricted   ·   Cilium NetworkPolicy",
                 fontName="Body-Bold", fontSize=8, fillColor=BLUE))

    # servis satırı
    sy = H-150
    svcs = [("frontend\n:8080", TEAL), ("catalog\n:8080", GREEN), ("query\n:8081", GREEN),
            ("ai\n:8082", GREEN), ("auth\n:8889", AMBER), ("mail\n:8890", AMBER)]
    n = len(svcs)
    sw = (W-2*pad-(n-1)*gap)/n
    sxs = []
    for i, (t, c) in enumerate(svcs):
        x = pad + i*(sw+gap)
        box(d, x, sy, sw, 38, t, c, fs=7.2)
        sxs.append(x+sw/2)
    svc_top, svc_bot = sy+38, sy

    # gateway -> servisler (ortogonal HTTPRoute barası)
    grail = H-86
    _vseg(d, cx, H-30, grail, SLATE, 1.1)
    _hseg(d, min(sxs), max(sxs), grail, SLATE, 1.0)
    for xc in sxs:
        arrow(d, xc, grail, xc, svc_top, SLATE, 0.9)
    d.add(String(cx, grail+4, "HTTPRoute (PathPrefix) · sertleştirilmiş pod: uid 65532 · readOnlyRootFS · seccomp",
                 fontName="Body-Italic", fontSize=6.5, fillColor=SLATE, textAnchor="middle"))

    # veri/altyapı satırı
    dy = 50
    stores = [("Bitnami PostgreSQL\n:5432", TEAL), ("Dragonfly\n:6379 (önbellek)", RED),
              ("NATS / JetStream\n:4222", PURPLE)]
    dw = (W-2*pad-2*gap)/3
    dxs = []
    for i, (t, c) in enumerate(stores):
        x = pad + i*(dw+gap)
        box(d, x, dy, dw, 36, t, c, fs=7.2)
        dxs.append(x+dw/2)
    ds_top = dy+36

    # backend servisleri -> veri katmanı (tek ortogonal bara)
    drail = 108
    backend = sxs[1:]  # frontend hariç
    for xc in backend:
        _vseg(d, xc, svc_bot, drail, SLATE, 0.7)
    _hseg(d, min(backend+dxs), max(backend+dxs), drail, SLATE, 0.9)
    for xc in dxs:
        arrow(d, xc, drail, xc, ds_top, SLATE, 1.0)
    d.add(String(pad, drail+4, "veri erişimi: metadata/oturum (PostgreSQL) · önbellek (Dragonfly) · iş kuyruğu (NATS)",
                 fontName="Body-Italic", fontSize=6.5, fillColor=SLATE, textAnchor="start"))

    # notlar
    d.add(String(pad, dy-14, "PreSync Job: migrate (sync-wave -10)   ·   ai pod’u NATS tüketicisini içinde çalıştırır (ayrı worker yok)   ·   "
                 "postgresql ayrıca LoadBalancer VIP (192.168.0.164:5432)",
                 fontName="Body-Italic", fontSize=6.4, fillColor=SLATE, textAnchor="start"))
    return d

# ---- Olgunluk skor kartı (yatay barlar) -----------------------------------
def diagram_scorecard():
    dims = [("Güvenlik", 4.7, GREEN), ("Test Kapsamı", 4.5, GREEN),
            ("Kod Kalitesi & Mimari", 4.2, GREEN), ("Gözlemlenebilirlik", 4.2, GREEN),
            ("Performans", 4.0, GREEN), ("AI/LLM Mühendisliği", 4.7, GREEN),
            ("DevX / Sürdürülebilirlik", 4.7, GREEN)]
    W = CONTENT_W
    rowh = 26
    H = len(dims)*rowh + 24
    d = Drawing(W, H)
    label_w = 150
    bar_x = label_w + 8
    bar_max = W - bar_x - 40
    y = H - rowh
    # ölçek çizgileri
    for s in range(1,6):
        gx = bar_x + bar_max*(s/5.0)
        d.add(Line(gx, 8, gx, H-rowh+8, strokeColor=colors.HexColor("#E2E8F0"), strokeWidth=0.5))
        d.add(String(gx, H-12, str(s), fontName="Body", fontSize=6.5, fillColor=SLATE, textAnchor="middle"))
    for name, val, c in dims:
        d.add(String(label_w, y+6, name, fontName="Body-Bold", fontSize=8,
                     fillColor=INK, textAnchor="end"))
        d.add(Rect(bar_x, y+2, bar_max, 12, fillColor=SLATE_L, strokeColor=None, rx=3, ry=3))
        d.add(Rect(bar_x, y+2, bar_max*(val/5.0), 12, fillColor=c, strokeColor=None, rx=3, ry=3))
        d.add(String(bar_x+bar_max+6, y+5, "%.1f" % val, fontName="Body-Bold",
                     fontSize=8, fillColor=c, textAnchor="start"))
        y -= rowh
    return d

# --------------------------------------------------------------------------
# Doküman şablonu (kapak + üstbilgi/altbilgi + TOC)
# --------------------------------------------------------------------------
class BiqlyDoc(BaseDocTemplate):
    def __init__(self, filename, **kw):
        BaseDocTemplate.__init__(self, filename, **kw)
        frame = Frame(ML, MB, CONTENT_W, PAGE_H-MT-MB, id="main")
        self.addPageTemplates([
            PageTemplate(id="cover", frames=[Frame(0,0,PAGE_W,PAGE_H,id="c")],
                         onPage=self._cover_bg),
            PageTemplate(id="body", frames=[frame], onPage=self._decorate),
        ])
    def _cover_bg(self, canv, doc):
        canv.saveState()
        canv.setFillColor(NAVY)
        canv.rect(0, PAGE_H-150*mm, PAGE_W, 150*mm, fill=1, stroke=0)
        canv.setFillColor(BLUE)
        canv.rect(0, PAGE_H-152*mm, PAGE_W, 3*mm, fill=1, stroke=0)
        # alt aksan
        canv.setFillColor(BLUE)
        canv.rect(0, 0, PAGE_W, 18*mm, fill=1, stroke=0)
        canv.setFillColor(GREEN)
        canv.rect(0, 18*mm, PAGE_W, 2*mm, fill=1, stroke=0)
        canv.restoreState()
    def _decorate(self, canv, doc):
        canv.saveState()
        # üst bilgi
        canv.setFont("Body", 7.5)
        canv.setFillColor(SLATE)
        canv.drawString(ML, PAGE_H-MT+8*mm, "Biqly — Teknik Mimari & Best-Practice Analiz Dokümanı")
        canv.setStrokeColor(GREY_LN); canv.setLineWidth(0.5)
        canv.line(ML, PAGE_H-MT+6*mm, PAGE_W-MR, PAGE_H-MT+6*mm)
        # alt bilgi
        canv.line(ML, MB-4*mm, PAGE_W-MR, MB-4*mm)
        canv.setFont("Body", 7.5); canv.setFillColor(SLATE)
        canv.drawString(ML, MB-8*mm, "Gizli · Yalnızca dahili kullanım")
        canv.drawRightString(PAGE_W-MR, MB-8*mm, "Sayfa %d" % doc.page)
        canv.restoreState()

    def afterFlowable(self, flowable):
        if isinstance(flowable, Heading):
            txt = flowable.getPlainText()
            if flowable.level == 0:
                self.notify("TOCEntry", (0, txt, self.page, flowable.bm_key))
            elif flowable.level == 1:
                self.notify("TOCEntry", (1, txt, self.page, flowable.bm_key))

# --------------------------------------------------------------------------
# İçerik
# --------------------------------------------------------------------------
story = []

# ---- KAPAK ----
story += [
    Spacer(1, 38*mm),
    Paragraph('<font color="white"><b>BIQLY</b></font>', S("ct", fontName="Body-Bold", fontSize=40, leading=44, textColor=colors.white)),
    Spacer(1, 4*mm),
    Paragraph('<font color="white">Teknik Mimari &amp; Best-Practice Analiz Dokümanı</font>',
              S("cs", fontName="Body", fontSize=17, leading=22, textColor=colors.white)),
    Spacer(1, 6*mm),
    Paragraph('<font color="#CBD5E1">Doğal dilden SQL üreten (Text-to-SQL) yapay zekâ destekli iş zekâsı platformu</font>',
              S("cs2", fontName="Body", fontSize=11, leading=16, textColor=colors.HexColor("#CBD5E1"))),
    Spacer(1, 78*mm),
    Paragraph("Kapsam: Go backend · React/TypeScript frontend · CI/CD pipeline’ları · Kubernetes dağıtımı · kalite denetimi",
              st_cover_meta),
    Spacer(1, 3*mm),
    Paragraph("Sürüm 2.0 · Haziran 2026 · Geliştirme-sonrası güncellenmiş sürüm · Hibrit (yönetici özeti + teknik derinlik)", st_cover_meta),
    Paragraph("Hazırlayan: Mimari Analiz · go.mod: github.com/biqly/biqly (Go 1.26) · 8 öneriden 7’si uygulandı", st_cover_meta),
    PageBreak(),
]

# ---- İÇİNDEKİLER ----
toc = TableOfContents()
toc.levelStyles = [st_toc1, st_toc2]
toc.dotsMinLevel = 0
story += [Heading("İçindekiler", st_h1, -1) if False else Paragraph("İçindekiler", st_h1),
          SP(6), toc, PageBreak()]

# ============================ 1. YÖNETİCİ ÖZETİ ============================
story += [H1("1. Yönetici Özeti")]
story += [P(
 "Biqly; kullanıcıların doğal dilde (Türkçe/İngilizce) sorduğu soruları güvenli, doğrulanmış SQL sorgularına "
 "çevirip çalıştıran, yapay zekâ destekli bir <b>iş zekâsı (BI) ve veri analitiği platformudur</b>. Sistem; "
 "Go ile yazılmış bir backend (modüler monolit / mikroservis hibridi), React&nbsp;19 + TypeScript ile yazılmış "
 "tek sayfa uygulaması (SPA) frontend ve Helm + ArgoCD ile yönetilen GitOps tabanlı bir Kubernetes dağıtımından "
 "oluşur.")]
story += [P(
 "Bu doküman iki katmanlıdır: önce karar vericilere yönelik üst seviye değerlendirme, ardından geliştiriciler "
 "ve mimarlar için teknik derinlik sunar. Her bölümde sistemin <b>nasıl çalıştığı</b> anlatıldıktan sonra "
 "<b>güçlü yönler, riskler ve somut iyileştirme önerileri</b> verilmiştir.")]

story += [H3("Öne çıkan tespitler")]
story += [BUL([
 "<b>AI/Text-to-SQL motoru platformun en güçlü ve farklılaştırıcı bileşenidir.</b> LLM doğrudan SQL değil, "
 "ara bir <i>LogicalQuery</i> üretir; bu sorgu semantik modele karşı doğrulanır ve ancak ondan sonra lehçeye "
 "özel SQL’e derlenir. EXPLAIN ile kuru-çalıştırma (dry-run), kendi kendini onaran yeniden deneme döngüsü, "
 "öz-tutarlılık oylaması ve güven skorlaması ile birlikte bu, tipik “prompt-and-pray” yaklaşımının çok üzerindedir.",
 "<b>Güvenlik katmanlı ve özenlidir:</b> RS256 JWT, zamanlama-güvenli CSRF, mutlak + boşta kalma oturum süreleri, "
 "AES-256-GCM ile şifrelenmiş sağlayıcı anahtarları, parametreli SQL ve beyaz-liste tabanlı sorgu doğrulaması.",
 "<b>Mühendislik disiplini yüksektir:</b> temiz <font name='Mono'>cmd/internal/pkg</font> ayrımı, modern Go&nbsp;1.26 "
 "hata deyimleri (148× <font name='Mono'>errors.Is</font>), ~55 linter, ESLint+knip+Prettier kapısı ve zorunlu pre-commit denetimleri.",
 "<b>Mimari — prod’da saf mikroservis (canlı küme ile doğrulandı):</b> 6 bağımsız servis Deployment’ı "
 "(ai/auth/catalog/query/frontend/mail) + Postgres StatefulSet + Dragonfly + NATS; Gateway API HTTPRoute’ları "
 "path’e göre doğrudan her servise yönlendirir. <font name='Mono'>cmd/api</font> all-in-one binary yalnızca "
 "local/docker-compose içindir; NATS tüketicisi ayrı worker pod’u değil, ai pod’u içinde çalışır.",
 "<b>Bu turda da iyileştirmeler sürdü:</b> 4 yüksek-karmaşıklık fonksiyonu LOW/MEDIUM’a indirildi "
 "(Compare 50→1, datasourceDraft 47→4, SyncMetadata 45→9, Validate 40→3), prod-tespiti tek "
 "<font name='Mono'>env.IsProduction()</font> yardımcısında birleştirildi, <b>catalog’daki kimliksiz-erişim açığı kapatıldı</b>, "
 "CI’ya <font name='Mono'>govulncheck</font> eklendi. Tek kısmi kalem hâlâ <font name='Mono'>AIConfig</font>: "
 "adlandırılmış alt-konfiglere geçirildi ama tanrı-nesnesi skoru değişmedi (21 alan, KRİTİK).",
])]

story += [H3("Genel olgunluk değerlendirmesi")]
story += [P("Biqly, bir prototip değil; <b>üretime hazır</b>, geç-aşama bir mikroservis kod tabanıdır. Bu doküman, önceki "
            "raporlardaki önerilerin uygulanmış halidir: ilk denetimdeki <b>8 boşluğun 7’si tamamen kapatıldı</b>, "
            "kalan AIConfig kalemi de büyük ölçüde ilerletildi. Tüm değişiklikler kodda doğrulandı "
            "(<font name='Mono'>go build ./...</font> ve <font name='Mono'>go vet</font> temiz). Ortalama olgunluk "
            "~2.5/5 bandından <b>4.4/5</b> seviyesine yükseldi. Aşağıdaki skor kartı yedi boyutu özetler:")]
story += [diagram_scorecard(), CAP("Şekil 1 — Mühendislik olgunluğu skor kartı (5 üzerinden; geliştirme sonrası, kanıta dayalı).")]
story += [SP(8)]
story += [H3("İyileştirme durumu — önceki rapora kıyasla")]
story += [mk_table(
 ["#", "Önceki denetimde işaretlenen boşluk", "Durum", "Kanıt"],
 [["1", "OTEL dağıtık izleme kodda yok", "<font color='%s'><b>● Kapatıldı</b></font>" % hx(GREEN),
   "3 servis main’inde provider, <font name='Mono'>otelhttp</font> + 3 span (ProcessQuestion / Compile / Execute); otel artık doğrudan bağımlılık"],
  ["2", "AI eval CI’da kapı değil", "<font color='%s'><b>● Kapatıldı</b></font>" % hx(GREEN),
   "<font name='Mono'>test.yml</font> + <font name='Mono'>ci.yml</font> eval-regression işi; 1.00 sayısal eşikler (<font name='Mono'>t.Fatalf</font>)"],
  ["3", "Lehçe sürücüleri & config/dashboard/queue ince test", "<font color='%s'><b>● Kapatıldı</b></font>" % hx(GREEN),
   "Sürücü başına 2 test; <font name='Mono'>scripts/coveragecheck</font> %85/%80 kapsam kapısı"],
  ["4", "<font name='Mono'>AIConfig</font> + yüksek-karmaşıklık fonksiyonları", "<font color='%s'><b>● Kısmi</b></font>" % hx(AMBER),
   "4 fonksiyon LOW/MEDIUM’a indi (Compare 50→1 …); AIConfig adlandırılmış alt-konfiglere geçti ama skor 60/KRİTİK değişmedi"],
  ["5", "HSTS varsayılan kapalı", "<font color='%s'><b>● Kapatıldı</b></font>" % hx(GREEN),
   "<font name='Mono'>BI_ENV=production</font> ile otomatik açık (fail-closed)"],
  ["6", "CSP / X-Frame-Options eksik", "<font color='%s'><b>● Kapatıldı</b></font>" % hx(GREEN),
   "CSP + X-Frame-Options + X-Content-Type-Options + COOP/CORP + Referrer-Policy"],
  ["7", "Kökte commit’li test binary’leri", "<font color='%s'><b>● Kapatıldı</b></font>" % hx(GREEN),
   "Diskte/git’te yok; <font name='Mono'>.gitignore</font> + <font name='Mono'>make clean</font>"],
  ["8", "ESLint uyarı tavanı + SQL inceleme bulguları", "<font color='%s'><b>● Kapatıldı</b></font>" % hx(GREEN),
   "640c3b6: parametreli metadata SQL; readonly muhafıza literal/yorum sıyırma eklendi"],
 ],
 [8*mm, 58*mm, 22*mm, CONTENT_W-88*mm])]
story += [PageBreak()]

# ============================ 2. SİSTEM GENEL BAKIŞI =======================
story += [H1("2. Sistem Genel Bakışı")]
story += [P(
 "Biqly’nin temel iş akışı şudur: bir kullanıcı bir veri kaynağı seçer ve doğal dilde bir soru sorar "
 "(örn. “geçen çeyrekte en çok satan 10 ürün”). Platform; ilgili tabloları otomatik olarak yönlendirir (table routing), "
 "bir semantik model bağlamı kurar, LLM’den yapısal bir mantıksal sorgu üretmesini ister, bu sorguyu doğrular ve "
 "lehçeye uygun SQL’e derleyip çalıştırır; sonuç ve üretilen SQL kullanıcıya geri döner.")]
story += [P(
 "Platform aynı zamanda; <b>semantik modelleme</b> (sürükle-bırak tuval), <b>veri kataloğu / metadata yönetimi</b>, "
 "<b>panolar (dashboards)</b>, <b>rol tabanlı erişim (RBAC)</b>, <b>PII tespiti ve maskeleme</b>, <b>denetim kayıtları</b> "
 "ve <b>AI değerlendirme (eval) koşum hattı</b> gibi kurumsal yetenekler sunar.")]

story += [H2("2.1 Teknoloji Yığını (Özet)")]
story += [mk_table(
 ["Katman", "Teknoloji / Kütüphane", "Not"],
 [["Backend dili", "Go 1.26.4", "Modül: github.com/biqly/biqly"],
  ["HTTP yönlendirme", "go-chi/v5, go-chi/cors", "Tüm servislerde ortak"],
  ["Veritabanı", "PostgreSQL (pgx/v5)", "Bitnami subchart · 3 mantıksal DB"],
  ["Harici veri kaynakları", "MySQL · SQL Server · ClickHouse", "Sürücü + lehçe soyutlaması"],
  ["Mesajlaşma", "NATS / JetStream", "Asenkron AI iş kuyruğu"],
  ["Önbellek", "Dragonfly (Redis uyumlu)", "Rate-limit, AI yanıt önbelleği, RBAC"],
  ["Kimlik/kripto", "JWT RS256 · WebAuthn · LDAP · OAuth", "golang-jwt, go-webauthn"],
  ["JSON (sıcak yol)", "bytedance/sonic · json-iterator", "Performans için"],
  ["Frontend", "React 19 · TypeScript 5.7 · Vite", "Vanilla CSS + BEM, i18n"],
  ["Grafik", "Recharts 3.8", "Pano ve görselleştirme"],
  ["Dağıtım", "Helm umbrella + ArgoCD", "GitOps · ghcr.io/biqly/*"],
  ["Gözlemlenebilirlik", "slog · Prometheus · OTEL/Jaeger", "OTLP-HTTP tracing kodda enstrümante (bkz. §9.4)"],
 ],
 [30*mm, 70*mm, CONTENT_W-100*mm])]
story += [SP(4)]
story += [Q("Mimari notu: <b>Prod ortamında sistem saf mikroservis olarak çalışır</b> — her servis kendi Deployment’ı, "
            "imajı ve portuyla; Gateway API HTTPRoute’ları istekleri path’e göre doğrudan ilgili servise yönlendirir "
            "(canlı küme ile doğrulandı, §8.3). Kod tabanı aynı <font name='Mono'>internal/</font> paketlerini paylaşacak "
            "şekilde modüler kurgulanmıştır; bu sayede tek bir <font name='Mono'>cmd/api</font> all-in-one binary tüm "
            "servisleri süreç-içi sunabilir — bu yalnızca <b>local/docker-compose geliştirme</b> içindir, prod’da kullanılmaz.")]
story += [PageBreak()]

# ============================ 3. MİMARİ GENEL BAKIŞ ========================
story += [H1("3. Mimari Genel Bakış")]
story += [P("Sistem prod ortamında <b>bağımsız mikroservislerden</b> oluşur. Bir <b>Kubernetes Gateway (Gateway API)</b>, "
            "tek dış host (<font name='Mono'>abi.il1.nl</font>) üzerinden gelen istekleri <b>PathPrefix</b> kurallarıyla "
            "doğrudan ilgili servise yönlendirir — araya giren merkezi bir BFF/proxy pod’u yoktur. Her servis kendi "
            "Deployment’ı olarak çalışır, JWT’yi sınırda doğrular ve servisler arası tüm çağrılar tipli HTTP+JSON "
            "istemcileriyle (<font name='Mono'>pkg/*client</font>) yalnızca küme içinden yapılır.")]
story += [diagram_architecture(),
          CAP("Şekil 2 — Prod mikroservis topolojisi: Gateway API path-yönlendirmesi ve servisler arası çağrı grafiği.")]

story += [H2("3.1 Prod Servisleri ve Çalıştırılabilir Binary’ler")]
story += [P("Prod’da <b>6 servis</b> ayrı Deployment olarak çalışır (her biri kendi imajı/portu). "
            "<font name='Mono'>api</font> ve <font name='Mono'>worker</font> binary’leri prod’da <b>dağıtılmaz</b> — "
            "<font name='Mono'>api</font> local all-in-one, NATS tüketicisi ise ai servisinin içine gömülüdür.")]
story += [mk_table(
 ["Servis (port)", "Sorumluluk", "Dağıtım"],
 [["catalog :8080", "Veri kaynakları, metadata, semantik modeller, panolar, izinler, drift.", "Deployment"],
  ["query :8081", "Sorgu motoru: mantıksal sorguyu derler / çalıştırır / EXPLAIN ile doğrular.", "Deployment"],
  ["ai :8082", "NL→sorgu hattı. <b>NATS iş tüketicisini süreç-içinde çalıştırır</b> (ayrı worker yok).", "Deployment"],
  ["auth :8889", "Kimlik: JWT üretimi/açık-anahtar, oturum, RBAC, MFA, LDAP, OAuth, magic-link.", "Deployment"],
  ["mail :8890", "İşlemsel e-posta (iç servis): SMTP, yeniden deneme, blok listesi.", "Deployment"],
  ["frontend :8080", "Statik React SPA (nginx).", "Deployment"],
  ["api · worker", "all-in-one BFF / başsız işçi — <b>yalnızca local/docker-compose</b>.", "Prod’da yok"],
  ["migrate · export-sft", "Şema göçleri (PreSync Job) ve SFT veri kümesi dışa aktarımı.", "Job / CLI"],
 ],
 [30*mm, CONTENT_W-30*mm-24*mm, 24*mm])]

story += [H2("3.2 Yönlendirme ve Servisler Arası İletişim")]
story += [BUL([
 "<b>Gateway API HTTPRoute (PathPrefix → servis):</b> <font name='Mono'>/</font>→frontend; "
 "<font name='Mono'>/api/datasources·metadata·semantic·permissions·dashboards</font>→catalog; "
 "<font name='Mono'>/api/query</font>→query; <font name='Mono'>/api/ai</font>→ai; <font name='Mono'>/api/auth</font>→auth.",
 "<b>ai servisi</b> → <font name='Mono'>BI_CATALOG_SERVICE_URL=http://biqly-catalog:8080</font>, "
 "<font name='Mono'>BI_QUERY_SERVICE_URL=http://biqly-query:8081</font> (catalogclient/queryclient).",
 "<b>Tüm servisler</b> → auth <font name='Mono'>/internal/auth/public-key</font> (JWT RSA açık anahtarı), "
 "<font name='Mono'>/check-permission</font>, <font name='Mono'>/check-datasource-access</font>.",
 "<b>auth</b> → mail (işlemsel e-posta gönderimi).",
 "<b>İç yüzey</b> (<font name='Mono'>/internal/*</font>) genel ağ geçidinden asla eşleşmez; <font name='Mono'>X-Internal-Token</font> ile korunur.",
])]
story += [PageBreak()]

# ============================ 4. BACKEND ANALİZİ ===========================
story += [H1("4. Backend Analizi (Go)")]
story += [P("Backend; 73 paket, ~593 dosya ve ~5.458 sembolden oluşur. Kod tabanı; sorumlulukları net biçimde ayıran "
            "<font name='Mono'>cmd/</font> (giriş noktaları), <font name='Mono'>internal/</font> (uygulama) ve "
            "<font name='Mono'>pkg/</font> (paylaşılan SDK) üçlüsüne dayanır.")]

story += [H2("4.1 internal/ Paketleri")]
story += [mk_table(
 ["Paket", "Sorumluluk"],
 [["ai", "Metni sorguya çeviren orkestratör (<font name='Mono'>Service.ProcessQuestion</font>). Alt paketler: prompt, provider, routing (tablo yönlendirme), eval, ambiguity, abtest, embedder, response_cache."],
  ["app", "Bağımlılık enjeksiyonu kökü. <font name='Mono'>NewDependencies/NewAIDependencies/...</font>; süreç-içi mi yoksa istemci-adaptör mü kararını verir."],
  ["auth", "Kimlik çekirdeği: Service, JWTManager (RS256), SessionManager, CSRF, RateLimiter; rbac, mfa (TOTP+WebAuthn), ldap, oauth, workspace."],
  ["core", "Servis seviyesi sorgu orkestrasyonu (Compile/Run/DryRun) ve tipli ServiceError."],
  ["query", "Mantıksal sorgu → SQL: Compiler, expr_compiler (AST→SQL + PII maskeleme), Validator, ComputeFingerprint."],
  ["semantic", "Semantik model alanı: Dimension/Metric/Join, CompositeModel, metric_graph (bağımlılık grafiği + döngü tespiti), drift."],
  ["datasource", "DB sürücü soyutlaması: Driver arayüzü, Registry, postgres/mysql/sqlserver/clickhouse, pool_cache."],
  ["dialect", "Motora özel SQL üretimi (CoreDialect/SampleDialect), tanımlayıcı tırnaklama."],
  ["security", "Encryption (AES), DSN redaksiyonu, güvenilir-proxy IP, RowFilter (satır seviyesi güvenlik), read-only SQL koruması, pii."],
  ["metadata", "Metadata Postgres deposu: datasources/tables/columns/relations, few-shot, sözlük, ai_query_history, ai_jobs."],
  ["queue", "AI iş kuyruğu soyutlaması: NATSQueue (JetStream, dead-letter) + LocalAIJobQueue (geliştirme)."],
  ["http", "Yönlendirme + handler’lar + middleware yığını (§7)."],
  ["audit", "Asenkron sorgu/kimlik denetim kaydı (tamponlu DBWriter)."],
  ["dbmigrate / config / i18n / mail / dashboard / platform / errmsg / semanticgen", "Sırasıyla: göç koşucusu, tipli konfig (+GOMEMLIMIT), TR/EN i18n, mail iç bileşenleri, pano deposu, db/redis/observability, kararlı hata kodları, metadata’dan otomatik model üretimi."],
 ],
 [30*mm, CONTENT_W-30*mm])]

story += [H2("4.2 Teknik Notlar")]
story += [BUL([
 "<b>LLM SDK bağımlılığı yok:</b> AI sağlayıcı istemcisi <font name='Mono'>internal/ai/provider</font> içinde elle yazılmış, "
 "OpenAI-uyumlu bir HTTP istemcisidir; sağlayıcı/model DB’den yönetilir ve sıcak-yeniden-yüklenir (<font name='Mono'>ProviderStore</font>).",
 "<b>Üç mantıksal veritabanı:</b> metadata, auth ve mail ayrı DSN’lerle izole edilir.",
 "<b>Graceful shutdown</b> tüm HTTP sunucularında (SIGINT/SIGTERM) ve yapılandırılmış <font name='Mono'>slog</font> ile loglama.",
])]
story += [PageBreak()]

# ============================ 5. FRONTEND ANALİZİ ==========================
story += [H1("5. Frontend Analizi (React / TypeScript)")]
story += [P("Frontend; AI destekli BI platformunu önyüzleyen tek sayfa bir React uygulamasıdır (~65 bin satır TS/TSX/CSS). "
            "Bilinçli olarak <b>bağımlılık-hafif</b> tasarlanmıştır: Redux/React-Query gibi global durum/veri-çekme "
            "kütüphaneleri yoktur — durum React Context + özel hook’lar, veri çekme elle yazılmış bir sarmalayıcı ile yapılır.")]

story += [H2("5.1 Uygulama Mimarisi ve Yönlendirme")]
story += [P("<font name='Mono'>main.tsx</font> iç içe sağlayıcı (provider) ağacını kurar: "
            "I18n → Theme → Confirm → Toast → Shortcuts → AIJobs → Router → Auth. Yönlendirme "
            "<b>react-router-dom v7</b> ile yapılır; iki üst dal vardır: <font name='Mono'>GuestGuard</font> altındaki "
            "misafir rotaları (giriş, kayıt, şifre sıfırlama) ve <font name='Mono'>AuthGuard</font> altındaki uygulama kabuğu.")]
story += [BUL([
 "<b>Kod bölme:</b> her rota <font name='Mono'>lazyWithPreload()</font> ile tembel yüklenir; kenar çubuğu "
 "fareyle üzerine gelince ilgili chunk’ı önceden ısıtır (preload).",
 "<b>Uygulama kabuğu:</b> daraltılabilir kenar çubuğu, ⌘K komut paleti, breadcrumb, workspace seçici, dil değiştirici, "
 "tema değiştirici ve rota başına <font name='Mono'>ErrorBoundary</font>.",
])]

story += [H2("5.2 Bileşen Envanteri (özet)")]
story += [mk_table(
 ["Alan", "Başlıca Bileşenler"],
 [["AI doğal dil sorgu / sohbet", "AIQuery + aiQuery/ (ChatPanel, AssistantMessageCard, RoutingPanel/routingViz, FeedbackSection)"],
  ["Semantik modelleme tuvali", "Modeling + modeling/ (ModelingCanvas, useModelingCanvas, canvasMath) — sıfırdan yazılmış sürükle/yakınlaştır tuval"],
  ["Sorgu oluşturucu", "QueryBuilder + adım bileşenleri (Fields/Filter/Summarize/Sort/Having/Window/Cte)"],
  ["Metadata / veri kataloğu", "Metadata, Datasources, TableBrowser, ResultTable, AI describe modal’ları"],
  ["Panolar & analitik", "Dashboard, DashboardBuilder (Recharts), AIUsageDashboard"],
  ["Değerlendirme (eval)", "Evaluation + EvalRun/Regression/History sekmeleri"],
  ["Kimlik", "auth/ (SignIn, SignUp, ForgotPassword, OAuthCallback, PasswordStrengthMeter, AuthProvider/Guard)"],
  ["Yönetim (admin)", "Admin sekmeli kabuğu: kullanıcı/rol/RLS/alan-izni/PII/LDAP/AI sağlayıcı/AB-deney/drift panelleri"],
 ],
 [42*mm, CONTENT_W-42*mm])]

story += [H2("5.3 API Katmanı, Durum ve i18n")]
story += [BUL([
 "<b>Fetch çekirdeği</b> (<font name='Mono'>apiClient.ts</font>): tipli <font name='Mono'>fetchJSON&lt;T&gt;</font>, "
 "AbortController zaman aşımı, otomatik <font name='Mono'>X-Locale</font> başlığı, Bearer token.",
 "<b>CSRF</b> (<font name='Mono'>csrf.ts</font>): tüm istekler <font name='Mono'>csrfFetch</font> üzerinden çift-gönderim deseniyle; "
 "güvenli metotlar token’ı yakalar, güvensiz metotlar <font name='Mono'>X-CSRF-Token</font> enjekte eder.",
 "<b>Durum &amp; kimlik:</b> global store yok — Context + hook’lar (AuthProvider, useApi, useAIJobs poller); erişim token’ı "
 "yalnızca bellekte, refresh token <font name='Mono'>localStorage</font>’da, 14 dk’da bir sessiz yenileme.",
 "<b>i18n:</b> kütüphanesiz, <b>derleme-zamanı tip-güvenli</b> çeviri anahtarları; diller TR (varsayılan) ve EN; "
 "admin/auth sözlükleri tembel yüklenir.",
 "<b>Stil:</b> Vanilla CSS + BEM; <font name='Mono'>index.css</font> tasarım-sistemi çekirdeği (CSS değişkenleri, light/dark tema), "
 "özellik bazlı ~38 CSS dosyası.",
])]
story += [PageBreak()]

# ============================ 6. TEMEL AKIŞLAR =============================
story += [H1("6. Temel Akışlar (Pipeline’lar)")]

story += [H2("6.1 Text-to-SQL Akışı")]
story += [P("Platformun kalbi budur. Giriş: <font name='Mono'>POST /api/ai/query</font> (önizleme) veya "
            "<font name='Mono'>/api/ai/query/run</font>. Ağ geçidi <font name='Mono'>ai:query</font> iznini ve veri kaynağı "
            "erişimini uygular; sonra istek AI hattına girer. Aşağıdaki adımlar bir sıralı pipeline oluşturur:")]
story += [vertical_flow([
 "Ayrıştır & yönlendir: soru çözülür; semantik model seçilir veya TableRouter\notomatik model kurar (anahtar kelime + embedding skoru, FK join-yolu BFS)",
 "Bağlam kur: few-shot örnekleri, iş sözlüğü, tablo örnek satırları,\nönceki konuşma turları, hedef lehçe, PII (yasaklı) alanlar",
 "Prompt üret: PromptTier=0; rune/token bütçesine göre kademeli bağlam",
 "Üretim (öz-tutarlılık opsiyonel): N aday, kademeli sıcaklık → çoğunluk oyu\n(fingerprint üzerinde)",
 "Ayrıştır & doğrula: JSON→LogicalQuery, normalize, Validator (model whitelist),\nopsiyonel SQL EXPLAIN kuru-çalıştırma",
 "Onarım döngüsü: hata varsa bağlam katmanını yükselt ve yeniden dene\n(maxRetries’e kadar)",
 "Derle & çalıştır: Compiler → lehçe SQL (parametreli + RowFilter) → çalıştır",
 "Kalıcı kıl: ai_query_history + fingerprint; güven ≥ 0.85 ise önbelleğe al",
], accent=GREEN, box_h=30, gap=11),
 CAP("Şekil 3 — Text-to-SQL kendi-kendini-onaran pipeline’ı. Güven skoru: 0.9 − 0.15·doğrulama_hatası − 0.10·yeniden_deneme.")]
story += [PageBreak()]

story += [H2("6.2 Kimlik Doğrulama (Auth) Akışı")]
story += [P("Token modeli: <b>RS256 JWT erişim token’ları</b> (auth tarafından üretilir) + <b>opak, hash’lenmiş, "
            "DB tabanlı refresh oturumları.</b>")]
story += [vertical_flow([
 "Login: rate-limit (Redis) + CSRF doğrulaması; parola / LDAP / OAuth /\nmagic-link / passkey ile kimlik doğrula",
 "MFA (gerekirse): MFA challenge JWT üret → istemci TOTP / WebAuthn /\nkurtarma kodu ile tamamlar",
 "Oturum üret: SessionManager opak token (hash’li) + mutlak/boşta TTL +\nmaksimum oturum sınırı",
 "JWT üret: GenerateTokenWithVerification (userID, roller, workspace,\nveri kaynakları) — Secure/HttpOnly/SameSite çerez",
 "Diğer servislerde doğrulama: JWTAuth → auth public-key’i çek+önbellekle →\nRS256/issuer/audience kontrolü",
 "RBAC: RequirePermission / RequireDatasourceAccess →\nauth /internal uçları (TTL önbellekli)",
], accent=AMBER, box_h=30, gap=11),
 CAP("Şekil 4 — Kimlik doğrulama ve yetkilendirme akışı (RS256 JWT + opak DB oturumu + RBAC).")]

story += [H2("6.3 Semantik Katman ve Derleme Pipeline’ı")]
story += [BUL([
 "<b>metric_graph.go:</b> kompozit modeller için metrik bağımlılık grafiği (<font name='Mono'>MetricNode/Edge</font>) "
 "kurulur; yayından/derlemeden önce <font name='Mono'>DetectCircularDependencies</font> ile döngü tespiti yapılır.",
 "<b>compiler.go:</b> LogicalQuery → CompiledQuery (SQL + args); join’ler join-grafiği üzerinden otomatik belirlenir; "
 "SELECT/WHERE/GROUP BY/HAVING/ORDER BY + pencere fonksiyonları kurulur. <b>Değerler asla string’e gömülmez (parametreli).</b>",
 "<b>expr_compiler.go:</b> ExprNode AST’i lehçe SQL’e çevrilir; <font name='Mono'>PIIMaskingConfig</font> ile PII maskeleme uygulanır.",
 "<b>validator.go:</b> alan/dimension/metric varlığı, tarih-filtre tipleri, satır üst sınırı kontrolleri; "
 "hatalar kararlı <font name='Mono'>errmsg</font> kodları taşır — AI onarım döngüsü bu kodları okur.",
 "<b>fingerprint.go:</b> sorgu + semantik model sürümü kanonikleştirilip kararlı hash üretilir; önbellek, geçmiş "
 "tekilleştirme ve öz-tutarlılık oylamasında kullanılır.",
])]
story += [PageBreak()]

# ============================ 7. HTTP & VERİ KATMANI =======================
story += [H1("7. HTTP Katmanı ve Veri Katmanı")]
story += [H2("7.1 HTTP Middleware Yığını")]
story += [P("Yığın (monolit Router): RequestID → trace-context yayılımı → istek logu → RealIP → chi Logger → "
            "Recoverer → SecurityHeaders (HSTS + CSP) → Locale → CORS (açık izin listesi). Auth servisi ek olarak "
            "CSRF ve RateLimiter ekler.")]
story += [P("<b>Rota grupları:</b> <font name='Mono'>/health, /ready, /metrics</font> (Prometheus); JWT korumalı "
            "<font name='Mono'>/api/*</font> (catalog, query, ai grupları, her rotada izin + veri kaynağı erişim kontrolü); "
            "küme-içi <font name='Mono'>/internal/*</font> (X-Internal-Token + denetim). Bir "
            "<font name='Mono'>BI_*_SERVICE_URL</font> ayarlandığında ilgili <font name='Mono'>/api</font> grubu "
            "ters-proxy ile değiştirilir.")]

story += [H2("7.2 Veri Katmanı")]
story += [mk_table(
 ["Bileşen", "Kullanım"],
 [["PostgreSQL (pgx/v5)", "Üç mantıksal DB: metadata, auth, mail. Harici kullanıcı veri kaynakları ayrıca MySQL/SQL Server/ClickHouse."],
  ["Göçler (dbmigrate)", "Numaralı .sql dosyaları, schema_migrations + dirty-state tespiti; migrate/auth-migrate/mail-migrate binary’leri."],
  ["NATS / JetStream", "Dayanıklı AI iş kuyruğu; yeniden teslim + dead-letter (maxDeliver). Uzun süren işler (batch describe, embedding)."],
  ["Dragonfly / Redis", "Rate-limit, AI yanıt önbelleği, RBAC veri-kaynağı erişim listeleri, oturum/OAuth durumu."],
  ["ai_jobs (metadata)", "İş durumu/faz/ilerleme, çakışma tespiti, atomik TryMarkAIJobRunning (tek-sefer talep), bayat-iş toplama."],
 ],
 [38*mm, CONTENT_W-38*mm])]
story += [PageBreak()]

# ============================ 8. CI/CD & DAĞITIM ===========================
story += [H1("8. CI/CD Pipeline’ları ve Dağıtım")]
story += [P("Monorepo; tek bir Kubernetes namespace’ine (<font name='Mono'>biqly</font>) Helm + ArgoCD GitOps ile dağıtılır. "
            "Konteyner kayıt defteri: <font name='Mono'>ghcr.io/biqly/*</font>. CI koşucu mimarisi ARM64’tür; imajlar "
            "<font name='Mono'>linux/arm64</font> olarak üretilir.")]
story += [diagram_cicd(),
          CAP("Şekil 5 — Commit’ten dağıtıma uçtan uca CI/CD ve GitOps pipeline’ı.")]

story += [H2("8.1 GitHub Actions İş Akışları")]
story += [mk_table(
 ["İş akışı", "Amaç / Aşamalar"],
 [["ci.yml", "Ana hat. backend (vet → test -race+cov → <b>AI eval regresyon kapısı</b> → tüm cmd/* derle), lint (golangci v2.12.2), frontend (npm run check = eslint+prettier+knip+vitest+build), docker-api & docker-frontend push."],
  ["test.yml", "Hafif kapı (PR’larda da): go test, golangci-lint, <b>helm lint + template</b>."],
  ["build-{ai,query,catalog,auth,mail,migrate}.yml", "Servis başına Docker imaj derleme/push (QEMU+buildx, linux/arm64). Etiket: <font name='Mono'>sha-&lt;commit&gt;</font>. migrate her main push’ında derlenir."],
  ["semgrep.yml", "SAST: p/security-audit, p/golang, p/react, p/typescript, p/owasp-top-ten → SARIF → fail-gate → GitHub Security. Push/PR + haftalık cron."],
 ],
 [42*mm, CONTENT_W-42*mm])]

story += [H2("8.2 Docker İmajları")]
story += [BUL([
 "<b>8 imaj</b>, hepsi çok-aşamalı, <font name='Mono'>linux/arm64</font>.",
 "<b>scratch tabanlı</b> (ai, query): yalnızca statik binary + CA paketi; <font name='Mono'>USER 65534</font>.",
 "<b>distroless tabanlı</b> (auth, mail): iki binary (servis + göç) + gömülü SQL göçleri; <font name='Mono'>nonroot</font>.",
 "<b>api</b>: <font name='Mono'>ARG SERVICE</font> ile çok-binary Dockerfile. <b>frontend</b>: Vite build → nginx:alpine (8080, SPA fallback + güvenlik başlıkları).",
])]
story += [PageBreak()]

story += [H2("8.3 Kubernetes Runtime Topolojisi")]
story += [diagram_k8s(),
          CAP("Şekil 6 — «biqly» namespace’indeki çalışma-zamanı dağıtım topolojisi.")]
story += [BUL([
 "<b>Helm umbrella chart:</b> namespace (Pod Security: restricted), Bitnami PostgreSQL, Dragonfly, NATS, "
 "Cilium NetworkPolicy’ler, gözlemlenebilirlik yığını (OTEL/Jaeger/Vector/Prometheus/Grafana/Alertmanager).",
 "<b>Sertleştirilmiş pod’lar:</b> <font name='Mono'>runAsNonRoot</font>, uid 65532, <font name='Mono'>readOnlyRootFilesystem</font>, "
 "tüm capability’ler düşürülür, seccomp RuntimeDefault, <font name='Mono'>wait-for-postgres</font> initContainer.",
 "<b>ArgoCD:</b> main dalından, <font name='Mono'>values-prod.yaml</font> ile otomatik senkron (prune + selfHeal); "
 "argocd-image-updater <font name='Mono'>sha-</font> etiketlerini git’e yazarak imajları otomatik yükseltir.",
 "<b>Göçler:</b> PreSync hook Job’ları (sync-wave ile sıralı); metadata göçü catalog imajının sha’sıyla kilit-adımda çalışır; "
 "ConfigMap <font name='Mono'>migrations.biqly.io/revision</font> anotasyonu senkron tetikler.",
])]
story += [PageBreak()]

# ============================ 9. BEST-PRACTICE DEĞERLENDİRMESİ =============
story += [H1("9. Best-Practice Değerlendirmesi")]
story += [P("Bu bölüm yedi mühendislik boyutunu kanıta dayalı olarak değerlendirir. Her boyut için güçlü yönler, "
            "riskler ve somut öneriler verilir. İşaretler: ")]
story += [Paragraph(
  '<font color="%s"><b>● Güçlü</b></font>&nbsp;&nbsp;'
  '<font color="%s"><b>● İzlenmeli</b></font>&nbsp;&nbsp;'
  '<font color="%s"><b>● Risk</b></font>' % (hx(GREEN), hx(AMBER), hx(RED)),
  st_body)]

def assess(title, badge, badge_color, strengths, risks, recs):
    blk = [H2(title)]
    blk.append(Paragraph('<font color="%s"><b>Değerlendirme: %s</b></font>' %
                         (hx(badge_color), badge), st_h3))
    blk.append(Paragraph("<b>Güçlü yönler.</b> " + strengths, st_body))
    blk.append(Paragraph("<b>Riskler / boşluklar.</b> " + risks, st_body))
    blk.append(Paragraph("<b>Öneriler.</b> " + recs, st_body))
    return blk

story += assess("9.1 Güvenlik", "ÇOK GÜÇLÜ (en güçlü boyut)", GREEN,
 "Asimetrik <b>RS256 JWT</b> (issuer/audience doğrulamalı, MFA için ayrı audience); çift-gönderim CSRF + "
 "<font name='Mono'>subtle.ConstantTimeCompare</font> (zamanlama-güvenli); <b>mutlak + boşta</b> oturum süreleri, "
 "hash’li refresh token, cihaz parmak-izi; MFA/TOTP, WebAuthn, RBAC, LDAP, OAuth; sağlayıcı anahtarları "
 "<b>AES-256-GCM</b> ile şifreli. <b>Bu sürümde eklenenler:</b> tam güvenlik-başlığı seti — CSP, X-Frame-Options (DENY), "
 "X-Content-Type-Options, Referrer-Policy, COOP/CORP; <b>HSTS prod’da otomatik açık</b> (<font name='Mono'>BI_ENV=production</font>, "
 "fail-closed). Read-only SQL muhafızı sertleştirildi: allow/deny-list + çoklu-ifade engeli + <b>literal/yorum sıyırma</b>. "
 "<b>Bu turda:</b> prod-tespiti tek <font name='Mono'>env.IsProduction()</font> yardımcısında birleştirildi (api+auth ortak, testli); "
 "<b>catalog’daki kimliksiz-erişim açığı kapatıldı</b> (<font name='Mono'>/api/*</font> rotaları middleware’siz mount ediliyordu → "
 "artık JWT zorunlu); JWT admin-bypass zamanlama-güvenli ve boş-anahtar fail-closed; super-admin JWT kapısı; "
 "Cilium egress (catalog/query→auth) düzeltildi; CI’ya <font name='Mono'>govulncheck</font> + manuel semgrep tetikleyici eklendi.",
 "<font name='Mono'>BI_AUTH_ENABLED=false</font> iken savunma kısmen ağ-güvenine dayanır. Read-only koruması hâlâ tam SQL "
 "parser değil — fakat birincil savunma LogicalQuery whitelist doğrulaması ve parametreli yürütmedir.",
 "(1) Prod’da <font name='Mono'>BI_AUTH_ENABLED</font>’ı zorunlu (fail-closed) bir invariant yapın. "
 "(2) <font name='Mono'>govulncheck</font>/semgrep taramalarını sürdürün.")
story += [PageBreak()]

story += assess("9.2 Test", "GÜÇLÜ — KAPSAM KAPISI EKLENDİ", GREEN,
 "200+ <font name='Mono'>_test.go</font> dosyası; yoğunluk tam da kritik yerlerde: handlers, ai, query, auth, semantic, security. "
 "Benchmark’lar; <b>race testi</b> zorunlu kapı; golden-file testleri; frontend vitest. <b>Bu sürümde eklenenler:</b> "
 "lehçe sürücüleri için testler (postgres/mysql/sqlserver/clickhouse her biri 2 dosya, dialect 4), config/dashboard/queue her biri 3 dosya; "
 "ve <b>zorunlu kapsam kapısı</b> — <font name='Mono'>scripts/coveragecheck</font> ile paket başına taban: dialect & sürücüler "
 "<b>%85</b>, config/dashboard <b>%80</b> (<font name='Mono'>test.yml</font>’deki <font name='Mono'>coverage</font> işi). "
 "Eval regresyon kapısı da sayısal eşiklerle CI’da (bkz. §9.6).",
 "<font name='Mono'>internal/queue</font> test aldı (3 dosya) ama kapsam-taban haritasında değil — test ediliyor fakat kapıya bağlı değil. "
 "Eval eşikleri determinist stub sağlayıcıyla 1.00; canlı-LLM doğruluk kayması ölçülmüyor.",
 "(1) <font name='Mono'>queue</font>’yu kapsam-taban haritasına ekleyin. (2) Periyodik (nightly) canlı-LLM eval koşusu ekleyerek "
 "gerçek doğruluk kaymasını izleyin.")

story += assess("9.3 Kod Kalitesi & Mimari", "GÜÇLÜ — TEK KISMİ KALEM", AMBER,
 "Temiz <font name='Mono'>cmd/internal/pkg</font> ayrımı; doğru kararlılık gradyanı. Hata yönetimi Go 1.26 ev-kurallarına "
 "güçlü uyum: <b>148× errors.Is, 11× errors.AsType, yalnızca 1 eski errors.As.</b> <b>Bu sürümde:</b> orkestratör "
 "<font name='Mono'>ProcessQuestion</font> ayrıştırıldı — <font name='Mono'>//nolint:gocyclo,gocognit,funlen</font> kaldırıldı, "
 "karmaşıklık skoru <b>12</b>’ye düştü; ~30 küçük yardımcı çıkarıldı (generateWithRetries, parseAndValidate, "
 "buildSuccessResponse vb.). Tüm ağaç <font name='Mono'>go build ./...</font> ve <font name='Mono'>go vet</font> ✓.",
 "<b>Bu turda 4 yüksek-karmaşıklık fonksiyonu LOW/MEDIUM’a indirildi</b>: <font name='Mono'>Detector.Compare</font> 50→1, "
 "<font name='Mono'>datasourceDraft</font> 47→4, <font name='Mono'>SyncMetadata</font> 45→9, <font name='Mono'>Validator.Validate</font> 40→3 "
 "(toplam nolint 53→49); <font name='Mono'>pgarray</font> paketi <font name='Mono'>lib/pq</font> bağımlılığını tekleştirdi.",
 "<font name='Mono'>AIConfig</font> adlandırılmış alt-konfiglere geçirildi (Query/Embedding/Translation/Routing/Ambiguity — "
 "gömülü→isimli), <b>ama tanrı-nesnesi skoru değişmedi</b> (21 alan / 13 metot / 93 dış çağrı = skor 60, hâlâ KRİTİK). "
 "Geri kalan çok-yüksek odaklar: <font name='Mono'>ValidateContext</font> (39), <font name='Mono'>ValidateComposite</font> (27), "
 "<font name='Mono'>PasswordPolicy.Validate</font> (25).",
 "(1) <font name='Mono'>AIConfig</font>’in üst-seviye alanlarını gerçekten <b>dışarı taşıyın</b> (rename değil, taşıma). "
 "(2) Kalan 3 yüksek-karmaşıklık fonksiyonunu kademeli ayrıştırın.")
story += [PageBreak()]

story += assess("9.4 Gözlemlenebilirlik", "İYİ — TRACING ARTIK ENSTRÜMANTE", GREEN,
 "Yapılandırılmış loglama (<font name='Mono'>log/slog</font>), <b>Prometheus</b> metrikleri, Helm’de tam yığın "
 "(OTEL collector, Jaeger, Vector, ServiceMonitor, Grafana, Alertmanager) ve <b>6 SLO-tarzı alarm kuralı</b>. "
 "<b>Bu sürümde önceki en büyük boşluk kapatıldı — OTEL dağıtık izleme uçtan uca kodda:</b> <font name='Mono'>otel</font> "
 "artık doğrudan bağımlılık (v1.44); servis giriş noktalarında <font name='Mono'>SetupTracing</font> ile OTLP-HTTP "
 "tracer provider (endpoint yoksa zarif no-op); router’da <font name='Mono'>otelhttp</font> ingress span’i; ve tam istenen "
 "<b>LLM→derle→çalıştır</b> yolunda adlandırılmış span’ler: <font name='Mono'>ai.ProcessQuestion</font>, "
 "<font name='Mono'>query.Compile</font>, <font name='Mono'>query.Execute</font>.",
 "Span kapsamı henüz sığ: veri-kaynağı sürücü çağrıları tek tek span’lenmiyor (yalnız executor sarmalıyor). "
 "Prometheus <font name='Mono'>Metrics</font> struct’ı (35 alan) gograph’ta HIGH işaretli.",
 "(1) Sürücü/DB çağrıları ve few-shot/embedding gibi alt-fazlara span ekleyerek izleme derinliğini artırın. "
 "(2) Span’lere kritik öznitelikleri (model, attempt, fingerprint) ekleyin.")

story += assess("9.5 Performans", "İYİ, ÖLÇÜME DAYALI", GREEN,
 "Veri-kaynağı <b>bağlantı havuzu önbelleği</b> + <font name='Mono'>singleflight</font> tekilleştirme (thundering-herd "
 "önler, benchmark’lı); sürüm-farkında <b>sorgu fingerprint</b>’i (semantik model değişince önbelleği geçersiz kılar); "
 "AI yanıt + ambiguity önbellekleri; Dragonfly önbellek katmanı; CLAUDE.md’de ciddi sıcak-yol disiplini (prealloc, "
 "strings.Builder, pprof) ve golangci ile zorlanan <font name='Mono'>prealloc/perfsprint</font>; sıcak-yol JSON için sonic/json-iterator.",
 "<b>Bu sürümde:</b> Prometheus etiket kardinalitesi sınırlandı (853739e) ve gecikme artık <b>span’lerle atfedilebilir</b> "
 "(LLM vs derle vs çalıştır — §9.4). <b>LLM gecikmesi</b> hâlâ baskın maliyet; öz-tutarlılık oylaması (N aday) tur "
 "sayısını çoğaltır. Bu turda yeniden benchmark yapılmadı.",
 "(1) Yeni span’lerden p99 gecikme dağılımını periyodik raporlayın. (2) Öz-tutarlılık N’inin konfig-kapılı ve makul "
 "prod varsayılanlı olduğundan emin olun.")
story += [PageBreak()]

story += assess("9.6 AI / LLM Mühendisliği", "OLGUN — AÇIK FARKLILAŞTIRICI", GREEN,
 "<b>İki aşamalı mimari:</b> LLM ham SQL değil <i>LogicalQuery</i> üretir; semantik modele karşı doğrulanır ve "
 "<i>sonra</i> lehçe SQL’e derlenir — bu hem doğru tasarım hem de enjeksiyon savunmasının temelidir. <b>Onarım döngüsü</b> "
 "(yapısal hata bağlamıyla yeniden prompt + kademeli bağlam katmanları), <b>EXPLAIN dry-run</b> doğrulama kapısı, "
 "<b>öz-tutarlılık oylaması</b>, <b>güven skorlaması</b>, <b>belirsizlik (ambiguity) işleme</b> ve gerçek bir <b>eval koşum "
 "hattı</b> (golden case + LLM judge + regresyon testi) ile <font name='Mono'>cmd/export-sft</font> ince-ayar geri-besleme "
 "döngüsü. DB’den yönetilen, AES-GCM şifreli sağlayıcı/model. "
 "<b>Bu sürümde:</b> eval/regresyon paketi artık <b>CI kapısı</b> (<font name='Mono'>test.yml</font> + <font name='Mono'>ci.yml</font>) ve "
 "<b>açık sayısal eşiklerle</b> zorlanıyor (golden + benchmark mantıksal/yürütme oranı, <font name='Mono'>t.Fatalf</font>); "
 "orkestratör <font name='Mono'>ProcessQuestion</font> ayrıştırıldı (§9.3).",
 "Eval eşikleri <b>determinist stub sağlayıcı</b> üzerinde 1.00 — yani harness/derleyici regresyonunu yakalar, canlı-LLM "
 "doğruluk kaymasını ölçmez.",
 "(1) Periyodik (nightly) canlı-LLM eval koşusu ekleyerek gerçek doğruluk kaymasını izleyin. (2) Eval kapsamını yeni "
 "lehçe/edge senaryolarıyla genişletin.")

story += assess("9.7 DevX / Sürdürülebilirlik", "MÜKEMMEL", GREEN,
 "<b>golangci-lint v2</b> ~55 linter (gosec, errorlint, contextcheck, bodyclose, rowserrcheck, sqlclosecheck, noctx, "
 "sloglint, fmt.Print* ve panic’i yasaklayan forbidigo); frontend kapısı CI-eşdeğeri (ESLint + jsx-a11y + security, "
 "Prettier, <b>knip</b> ölü-kod tespiti, vitest, build); <font name='Mono'>deadcode -test</font> Go kapısında; "
 "zorunlu pre-commit denetimleri; gerçek i18n; 8 temiz <font name='Mono'>cmd/</font> giriş noktası; ADR dizini. "
 "<b>Bu sürümde:</b> repo kökü temizlendi — <font name='Mono'>*.test</font> ve <font name='Mono'>coverage.out</font> hem "
 "diskte hem git’te yok, <font name='Mono'>.gitignore</font> + <font name='Mono'>make clean</font> ile garanti; CI odaklı "
 "işlere bölündü (test / eval / coverage / lint / helm); yeni <font name='Mono'>make</font> hedefleri "
 "(<font name='Mono'>coverage-gate</font>, <font name='Mono'>eval-regression</font>).",
 "Kayda değer açık DevX riski kalmadı; küçük artık: bazı paketler kapsam-taban haritası dışında (örn. queue).",
 "(1) Kapsam-taban haritasını kademeli genişletin. (2) ESLint uyarı tavanını zamanla 0’a doğru sıkın.")
story += [PageBreak()]

# ============================ 10. SONUÇ ====================================
story += [H1("10. Sonuç ve Yol Haritası")]
story += [P("Biqly; canlı kümede <b>saf mikroservis</b> olarak çalışan (her servis ayrı Deployment, Gateway API path-yönlendirmesi), "
            "disiplinli, katmanlı-güvenlikli ve <b>standardın üzerindeki AI/text-to-SQL motoruyla</b> üretime hazır, geç-aşama "
            "bir kod tabanıdır. Bu doküman, önceki raporlardaki önerilerin uygulanmış halidir: ilk denetimdeki <b>8 boşluğun "
            "7’si tamamen kapatıldı</b>; bu turda ayrıca 4 yüksek-karmaşıklık fonksiyonu indirildi, prod-tespiti birleştirildi ve "
            "catalog’daki kimliksiz-erişim açığı kapatıldı. Tüm değişiklikler kodda doğrulandı "
            "(<font name='Mono'>go build ./...</font> ve <font name='Mono'>go vet</font> temiz).")]
story += [H3("Kalan öneriler (azalan öncelik)")]
story += [mk_table(
 ["Öncelik", "Aksiyon", "Etki"],
 [["<b>Orta</b>", "<font name='Mono'>AIConfig</font>’in üst-seviye alanlarını gerçekten dışarı taşı (rename değil)", "Tek kalan KRİTİK tanrı-nesnesini kapatır"],
  ["Orta", "Kalan yüksek-karmaşıklık fonksiyonları (ValidateContext 39 · ValidateComposite 27 · PasswordPolicy.Validate 25)", "Bakım borcu, test edilebilirlik"],
  ["Orta", "Periyodik (nightly) <b>canlı-LLM</b> eval koşusu", "Stub-determinist eşiğin ötesinde gerçek doğruluk kayması"],
  ["Orta", "Prod’da <font name='Mono'>BI_AUTH_ENABLED</font>’ı zorunlu invariant yap", "Ağ-güvenine düşmeyi engeller (fail-closed)"],
  ["Düşük", "İzleme derinliği (sürücü/DB span’leri); flaky <font name='Mono'>TestMFABypassCodeFlow</font> izolasyonu", "Gözlem granülerliği / CI kararlılığı"],
  ["Düşük", "<font name='Mono'>queue</font>’yu kapsam-taban haritasına ekle", "Hijyen / tutarlılık"],
 ],
 [22*mm, CONTENT_W-22*mm-58*mm, 58*mm])]
story += [SP(10)]
story += [Q("Bu doküman; kaynak kod statik analizine (gograph), doğrudan kaynak incelemesine, git geçmişi doğrulamasına "
            "ve <b>canlı Kubernetes kümesinin kubectl ile incelenmesine</b> dayanır. Skor kartı, geliştirme-sonrası kanıta "
            "dayalı niteliksel değerlendirmenin özetidir (ortalama ~4.4/5).")]

# --------------------------------------------------------------------------
# Derle (çok geçişli — TOC için)
# --------------------------------------------------------------------------
doc = BiqlyDoc(OUT, pagesize=A4, title="Biqly — Teknik Mimari & Best-Practice Analiz Dokümanı",
               author="Mimari Analiz", subject="Biqly mimari ve kalite analizi")
# kapak ilk şablonu kullansın, sonrası body
def _kickoff(canv, doc_):
    pass
# ilk sayfa cover template, sonra body'ye geç
from reportlab.platypus import NextPageTemplate
final = [NextPageTemplate("cover")] + [story[0]] if False else None

# story başına template yönetimi: ilk flowable'lar kapak; PageBreak sonrası body
full = [NextPageTemplate("body")] + story
# kapak için ilk template'i cover yapalım: en başa cover seçimi koy
full = [NextPageTemplate("cover")] + [Spacer(0,0)] + [NextPageTemplate("body")] + story

doc.multiBuild(full)
print("OK ->", OUT)
