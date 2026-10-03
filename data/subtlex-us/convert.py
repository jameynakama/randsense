"""Convert the SUBTLEX-US part-of-speech workbook to subtlex-us-pos.tsv.gz.

Download subtlexus1.zip from
https://www.ugent.be/pp/experimentele-psychologie/en/research/documents/subtlexus
and unzip it here, then run:

    uv run --with openpyxl data/subtlex-us/convert.py

Each output row is a word, a part of speech, and how many times the word was
tagged as that part of speech in the 51-million-word corpus.
"""

import gzip
from pathlib import Path

import openpyxl

HERE = Path(__file__).parent
SOURCE = HERE / "SUBTLEX-US frequency list with PoS and Zipf information.xlsx"
OUT = HERE / "subtlex-us-pos.tsv.gz"


def main() -> None:
    rows = openpyxl.load_workbook(SOURCE, read_only=True).active.iter_rows(values_only=True)
    header = next(rows)
    word_col = header.index("Word")
    pos_col = header.index("All_PoS_SUBTLEX")
    freqs_col = header.index("All_freqs_SUBTLEX")

    with gzip.open(OUT, "wt", newline="\n") as out:
        out.write("word\tpos\tcount\n")
        for row in rows:
            tags, counts = str(row[pos_col]), str(row[freqs_col])
            if tags == "#N/A":
                continue
            for tag, count in zip(tags.split("."), counts.split(".")):
                out.write(f"{row[word_col]}\t{tag}\t{count}\n")


if __name__ == "__main__":
    main()
