# Data Model and Reconciliation Specification

## Core model
Work -> Expression -> Edition -> MarketListing.

## Person resolution
Strong signals:
- VIAF/ISNI/Wikidata/authority exact links
- explicit same-as

Supporting:
- normalized names/aliases
- birth/death
- overlapping works
- co-contributors

Never merge common names solely by string equality.

## Edition resolution
Strong:
- valid ISBN where semantic scope matches
- authoritative control number
- source same-as

Supporting:
- title/subtitle
- contributor identities
- publisher/imprint
- publication date
- language
- format
- edition statement
- narrator/duration for audio

Contradictions must block auto-merge.

## Series
Store series independently with:
- aliases/localizations
- parent series
- series type
- work membership
- numeric and text position
- membership evidence/confidence

Differentiate series, subseries/era, universe/franchise, omnibus, collection, publisher line.

## Canonical selection
Source authority * field reliability * record-specificity * identity confidence * corroboration * freshness.

Missing values do not vote against known values.

## Benchmark corpus
Must include:
- common-name authors
- pseudonyms
- translations
- revised editions
- ebook/print variants
- different audiobook narrations
- abridged/unabridged
- omnibus
- novellas/fractional series order
- anthologies
- malformed/reused ISBNs
- CJK/RTL/Unicode aliases
