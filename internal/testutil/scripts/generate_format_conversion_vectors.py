"""Pin notification conversion whitespace against upstream Apprise 2.1.0."""
import argparse
import json
from pathlib import Path

from apprise import __version__
from apprise.conversion import html_to_markdown, markdown_to_html

ROOT = Path(__file__).resolve().parents[3]
OUTPUT = ROOT / "internal/notify/testdata/format_conversion.json"
MARKDOWN = [
    ("punctuation", "Don't change 'quoted' text or -- dashes..."),
    ("tab_stops", "Q:\tQuestion\nA:\tAnswer"),
    ("paragraphs", "first\nline\n\nsecond"),
    ("code", "`inline`"),
    ("code_newlines", "```\nfirst\nsecond\n```"),
    ("code_language", "```go\nif x > 0 { return x }\n```"),
    ("code_long_run", "````a ``` b````"),
    ("code_unclosed", "```\nopen"),
    ("code_space", "` a `"),
    ("strike", "~~strike~~ **bold** _emphasis_"),
    ("headings", "### heading\n\nbody\n\n### next"),
    ("html", "<b>bold</b>\nnext"),
]
HTML = [
    ("headings", "<h1>first</h1><h2>second</h2>"),
    ("paragraph_heading", "<p>paragraph</p>\n\n<h3>heading</h3>"),
    ("heading_pre", "<h3>heading</h3>\n\n<pre>\nfirst\nsecond\n</pre>"),
    ("paragraph_pre", "<p>paragraph</p>\n<pre>code</pre><p>after</p>"),
    ("pre_blank_lines", "<pre>\nfirst\n\nsecond\n</pre>"),
    ("pre_nested_code", "<pre><code>\nfirst\nsecond\n</code></pre>"),
    ("pre_backticks", "<pre>a ``` b\nnext</pre>"),
    ("pre_attributes", '<PRE class="x > y">\ncode\n</PRE>'),
    ("pre_multiple", "<pre>\none</pre><pre>\ntwo</pre>"),
    ("pre_marker_collision", "<p>\ue000apprise-pre\ue001</p><pre>\ncode</pre>"),
    ("inline_spaces", "<b>one</b> <i>two</i>"),
]

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    if __version__ != "2.1.0":
        raise SystemExit("requires pinned Apprise 2.1.0")
    cases = []
    for source, target, converter, inputs in [
        ("markdown", "html", markdown_to_html, MARKDOWN),
        ("html", "markdown", html_to_markdown, HTML),
    ]:
        for name, text in inputs:
            cases.append(dict(name=source+"/"+name, source=source, target=target,
                              input=text, expected=converter(text)))
    data = json.dumps(cases, indent=2, ensure_ascii=False)+"\n"
    if args.check:
        if OUTPUT.read_text() != data:
            raise SystemExit("format conversion vectors need regeneration")
    else:
        OUTPUT.write_text(data)

if __name__ == "__main__":
    main()
