Guida alla sintassi ISIS per `--isis-query`

Il portale ARS (motore Icaro/ISIS) accetta espressioni di ricerca strutturate. La CLI le
costruisce automaticamente dai flag (`--firmatario`, `--materia`, `--data`, …), ma con
`--isis-query` puoi passare un'espressione grezza al motore, sfruttando tutta la potenza ISIS.

Fonte: guida ufficiale https://dati.ars.sicilia.it/home/cerca/help.jsp — qui in forma operativa,
con le sigle di campo verificate su questa CLI.

## Concetti base

- Un'espressione è fatta di **termini** legati da **operatori di relazione**.
- Un termine può essere **qualificato** su uno o più campi: `termine.SIGLA` (solo in quel campo),
  oppure `termine:SIGLA` (in tutti i campi TRANNE quello).
- Le **parentesi** definiscono la priorità: `(occupazione giovanile).TITOL,TESTO`.
- I numeri vanno senza separatori delle migliaia. Le **date** sono numeriche `AAMMGG`
  (es. 25 feb 2026 → `260225`).

## Sigle di campo verificate

| Flag CLI | Archivio | Sigla ISIS | Note |
|---|---|---|---|
| `--legisl` | tutti | `LEGISL` | es. `18.LEGISL` |
| `--firmatario` | ddl, interrogazioni, interpellanze, mozioni, odg, risoluzioni | `FIRMAT` | `Cracolici.FIRMAT` |
| `--materia` | ddl | `SETTOR` | vocabolario: `ddl materie` |
| `--rubrica` | interrogazioni, interpellanze, mozioni, odg | `RUBRIC` | |
| `--anno` | leggi | `LEGANN` | valore letterale (anno intero) |
| `--anno` | resoconti | `ANNSED` | valore letterale (anno intero) |
| `--anno` | ddl | `DATPRE` | ddl non ha un campo anno: la CLI converte l'anno intero in un range `AAMMGG1/AAMMGG2` (1° gen - 31 dic) su `DATPRE` |
| `--numero` | ddl | `NUMDDL` | |
| `--numero` | leggi | `LEGNUM` | |
| `--numero` | interrogazioni/… | `NUMORD` | |
| `--numero` | resoconti | `NUMSED` | |
| `--data` | resoconti, convocazioni, sommari | `DATSED` | formato `AAMMGG`; range `AAMMGG/AAMMGG` |
| `--data` | ddl, interrogazioni, interpellanze, mozioni, odg, risoluzioni | `DATPRE` | presentazione, `AAMMGG`; range `AAMMGG/AAMMGG`. Esposto come flag su `deputato profilo`; sugli altri comandi `*/cerca` va passato via `--isis-query`. Su ddl `--anno` (sopra) resta la via comoda per l'anno intero |
| `--commissione` | convocazioni, sommari, pareri, risoluzioni | `COMMIS` | nome: `SESTA.COMMIS` |
| `--presidente` | sommari | `PRESID` | |
| `--autore` | biblioteca | `AUTORE` | |
| `--titolo` | biblioteca | `TITOLO` | |
| `--soggetto` | biblioteca | `SOGGET` | |
| (nessuno) | ddl | `P010`, `P012` | legge di destinazione: `alr <anno> nlr <numero>`. Vedi sotto |

Nota: la **commissione** si cerca per nome ordinale (`PRIMA`…`SESTA`) sul campo `COMMIS`; il
codice numerico `CODCOM` non è indicizzato. La CLI mappa automaticamente `--codcom 6` → `SESTA`.

### Dalla legge al DDL d'origine (`P010`/`P012`)

Un DDL confluito in legge registra la legge di destinazione nei campi `P010`/`P012`, nella
forma `alr <anno> nlr <numero>` (**a**nno **l**egge **r**egionale, **n**umero **l**egge
**r**egionale). È il legame **autorevole** fra legge e disegno di legge: lo stesso che la
scheda-legge del portale espone col link «DDL ed Iter».

```bash
# Da quale DDL nasce la L.R. 1 del 2024 (legge di stabilità)? -> ddl 638
ars-sicilia-pp-cli ddl cerca \
  --isis-query "alr adj 2024.P010,P012 sfrase nlr adj 1.P010,P012"
```

`legge cronologia` costruisce da sé questa query: usa il comando, non la query grezza. La
sigla serve se vuoi partire dai DDL (es. tutti i ddl confluiti in legge nel 2024:
`alr adj 2024.P010,P012`). Non cercare il DDL d'origine per **titolo**: i titoli si ripetono
ogni anno (ogni «Legge di stabilità regionale») e si aggancia l'anno sbagliato.

## Operatori (verificati ✓)

| Operatore | Sinonimi | Significato | Esito test |
|---|---|---|---|
| `E` | `AND` `ET` `UND` | entrambi i termini nel documento | ✓ |
| `O` | `OR` `OU` `ODER` | almeno uno | (doc) |
| `XOR` | `ONONE` | uno o l'altro ma non entrambi | (doc) |
| `NOT` | `NO` `ESCLUSO` `MENO` `EXCLU` | il primo sì, il secondo no | ✓ |
| `WITH` | `SFRASE` | stesso documento, stesso campo, stessa frase | (doc) |
| `SAME` | `SPARA` | stesso documento, stesso campo | (doc) |
| `NEAR`x | `VICINO`x | a distanza max x termini (x=1-9) | (doc) |
| `ADJ`x | `SEGUITO`x | il primo segue immediatamente il secondo | (doc) |

`✓` = testato su questa CLI; `(doc)` = documentato dal portale, non ancora testato qui.

## Termini speciali

- **Radice**: `LEG$` → LEGGE, LEGGIO, LEGISLATORE… (`$2`..`$9` limita l'estensione).
- **Desinenza**: `ZIONE%` → LEZIONE, RAZIONE…
- **Intervallo numerico**: `13/15.NUMDDL` (DDL dal 13 al 15).
- **Immagine esatta** (case-sensitive): `IMG(Rossi)`.
- **Tutti i documenti**: `ALLDOC` (utile per esclusioni: `ALLDOC NOT regole:TITOL`).
- **Select su campi formattati**: `SEL(NUMDDL *GT "500")` (operatori `*LT *LE *GT *GE *RG`).

## Esempi pronti

```bash
# DDL di materia Sanità ma NON firmati da Cracolici
ars-sicilia-pp-cli ddl cerca --legisl 18 \
  --isis-query "(18.LEGISL E Sanità.SETTOR) NOT Cracolici.FIRMAT"

# Interrogazioni con "pronto" adiacente a "soccorso"
ars-sicilia-pp-cli interrogazioni cerca --legisl 18 \
  --isis-query "(18.LEGISL E (pronto ADJ soccorso))"

# Resoconti d'aula in un intervallo di date
ars-sicilia-pp-cli resoconti cerca --legisl 18 \
  --isis-query "(18.LEGISL E 260224/260225.DATSED)"

# DDL governativi (iniziativa)
ars-sicilia-pp-cli ddl cerca --legisl 18 \
  --isis-query "(18.LEGISL E Governativa.FIRMAT)"
```

## Equivalenze con i flag nativi

Molte query non richiedono `--isis-query`: la CLI le costruisce dai flag.

| Obiettivo | Flag nativi | ISIS equivalente |
|---|---|---|
| DDL di un firmatario | `--firmatario Cracolici` | `Cracolici.FIRMAT E 18.LEGISL` |
| DDL per materia | `--materia Sanità` | `18.LEGISL E Sanità.SETTOR` |
| Resoconti per data | `--data 2026-02-25` | `260225.DATSED E 18.LEGISL` |
| Resoconti per intervallo | `--data 2026-02-24:2026-02-25` | `260224/260225.DATSED E 18.LEGISL` |
| Commissione per codice | `--codcom 6` | `SESTA.COMMIS E 18.LEGISL` |
| Escludere un termine | `--escludi ospedale` | `(…) NOT (ospedale)` |

L'esclusione **field-qualificata** (`NOT Cracolici.FIRMAT`) funziona solo via `--isis-query`:
`--escludi` lavora sul termine libero, in tutto il documento.
