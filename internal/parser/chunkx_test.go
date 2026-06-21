package parser

import (
	"testing"

	"github.com/gomantics/chunkx"
	"github.com/gomantics/chunkx/languages"
	"github.com/minhhh/grokdocs/internal/project"
)

const pythonSample = `import os
import sys

def greet(name):
    """Greet someone."""
    return f"Hello, {name}!"


class Calculator:
    """A simple calculator."""

    def add(self, a, b):
        return a + b

    def subtract(self, a, b):
        return a - b


def main():
    calc = Calculator()
    print(calc.add(1, 2))
`

const pythonSampleLong = `"""A module-level docstring for a larger Python sample.

This module demonstrates various Python constructs for chunking tests.
"""

import os
import sys
import json
import math
import hashlib
import itertools
from dataclasses import dataclass
from typing import Optional, List, Dict, Any, Tuple, Set, Callable
from collections import defaultdict, Counter
from datetime import datetime, timedelta


def greet(name: str, excited: bool = False) -> str:
    """Greet someone with an optional excited tone."""
    msg = f"Hello, {name}!"
    if excited:
        msg = msg.upper()
    return msg


def factorial(n: int) -> int:
    """Compute factorial recursively."""
    if n <= 1:
        return 1
    return n * factorial(n - 1)


def fibonacci(count: int) -> List[int]:
    """Generate fibonacci sequence up to count terms."""
    seq = []
    a, b = 0, 1
    for _ in range(count):
        seq.append(a)
        a, b = b, a + b
    return seq


def sieve_of_eratosthenes(limit: int) -> List[int]:
    """Find all prime numbers up to the given limit."""
    if limit < 2:
        return []
    is_prime = [True] * (limit + 1)
    is_prime[0] = is_prime[1] = False
    for p in range(2, int(limit ** 0.5) + 1):
        if is_prime[p]:
            for multiple in range(p * p, limit + 1, p):
                is_prime[multiple] = False
    return [i for i, prime in enumerate(is_prime) if prime]


def word_frequency(text: str) -> Dict[str, int]:
    """Count word frequency in a text string."""
    cleaned = text.lower()
    for ch in ".,!?;:()[]{}'\"-":
        cleaned = cleaned.replace(ch, " ")
    words = [w for w in cleaned.split() if w]
    return dict(Counter(words))


def levenshtein_distance(s1: str, s2: str) -> int:
    """Compute the Levenshtein edit distance between two strings."""
    if len(s1) < len(s2):
        return levenshtein_distance(s2, s1)
    if len(s2) == 0:
        return len(s1)
    prev = list(range(len(s2) + 1))
    for i, c1 in enumerate(s1):
        curr = [i + 1]
        for j, c2 in enumerate(s2):
            cost = 0 if c1 == c2 else 1
            curr.append(min(curr[j] + 1, prev[j + 1] + 1, prev[j] + cost))
        prev = curr
    return prev[len(s2)]


@dataclass
class Point:
    """A 2D point."""
    x: float
    y: float

    def distance_to(self, other: "Point") -> float:
        dx = self.x - other.x
        dy = self.y - other.y
        return (dx * dx + dy * dy) ** 0.5

    def midpoint(self, other: "Point") -> "Point":
        return Point((self.x + other.x) / 2, (self.y + other.y) / 2)

    def as_tuple(self) -> Tuple[float, float]:
        return (self.x, self.y)


class Calculator:
    """A simple calculator."""

    def add(self, a, b):
        return a + b

    def subtract(self, a, b):
        return a - b

    def multiply(self, a, b):
        return a * b

    def divide(self, a, b):
        if b == 0:
            raise ValueError("division by zero")
        return a / b

    def power(self, base, exp):
        return base ** exp

    def modulo(self, a, b):
        if b == 0:
            raise ValueError("division by zero")
        return a % b


class DataProcessor:
    """Processes data with various transformations."""

    def __init__(self, data: List[Dict[str, Any]]):
        self.data = data
        self._validate()

    def _validate(self):
        if not self.data:
            raise ValueError("data cannot be empty")

    def filter_by(self, key: str, value: Any) -> "DataProcessor":
        filtered = [d for d in self.data if d.get(key) == value]
        return DataProcessor(filtered)

    def sort_by(self, key: str, reverse: bool = False) -> "DataProcessor":
        sorted_data = sorted(self.data, key=lambda d: d.get(key, ""), reverse=reverse)
        return DataProcessor(sorted_data)

    def group_by(self, key: str) -> Dict[str, "DataProcessor"]:
        groups = defaultdict(list)
        for d in self.data:
            groups[d.get(key, "unknown")].append(d)
        return {k: DataProcessor(v) for k, v in groups.items()}

    def map_fields(self, field_map: Dict[str, str]) -> "DataProcessor":
        mapped = []
        for d in self.data:
            mapped.append({new: d.get(old, "") for old, new in field_map.items()})
        return DataProcessor(mapped)

    def to_json(self) -> str:
        return json.dumps(self.data, indent=2)

    def to_csv(self) -> str:
        if not self.data:
            return ""
        headers = list(self.data[0].keys())
        lines = [",".join(headers)]
        for d in self.data:
            lines.append(",".join(str(d.get(h, "")) for h in headers))
        return "\n".join(lines)


class ReportGenerator:
    """Generates formatted reports."""

    HEADER = "=" * 60

    def __init__(self, title: str):
        self.title = title
        self.sections: List[str] = []

    def add_section(self, heading: str, body: str):
        section = f"\n{heading}\n{'-' * len(heading)}\n{body}"
        self.sections.append(section)

    def add_table(self, headers: List[str], rows: List[List[str]]):
        col_widths = [len(h) for h in headers]
        for row in rows:
            for i, cell in enumerate(row):
                if i < len(col_widths):
                    col_widths[i] = max(col_widths[i], len(cell))
        lines = [" | ".join(h.ljust(col_widths[i]) for i, h in enumerate(headers))]
        lines.append("-+-".join("-" * w for w in col_widths))
        for row in rows:
            lines.append(" | ".join(row[i].ljust(col_widths[i]) for i in range(len(headers))))
        self.add_section("Table", "\n".join(lines))

    def render(self) -> str:
        parts = [self.HEADER, self.title, self.HEADER]
        parts.extend(self.sections)
        return "\n".join(parts)


class CacheManager:
    """Simple in-memory cache with TTL support."""

    def __init__(self, default_ttl: int = 300):
        self._store: Dict[str, Any] = {}
        self._expires: Dict[str, datetime] = {}
        self.default_ttl = default_ttl

    def get(self, key: str) -> Optional[Any]:
        if key not in self._store:
            return None
        if datetime.now() > self._expires.get(key, datetime.max):
            del self._store[key]
            del self._expires[key]
            return None
        return self._store[key]

    def set(self, key: str, value: Any, ttl: Optional[int] = None):
        self._store[key] = value
        self._expires[key] = datetime.now() + timedelta(seconds=ttl or self.default_ttl)

    def clear(self):
        self._store.clear()
        self._expires.clear()

    def size(self) -> int:
        return len(self._store)


def main():
    calc = Calculator()
    print(f"3 + 5 = {calc.add(3, 5)}")
    print(f"10 - 4 = {calc.subtract(10, 4)}")
    print(f"7 * 6 = {calc.multiply(7, 6)}")
    print(f"20 / 4 = {calc.divide(20, 4)}")

    p1 = Point(0, 0)
    p2 = Point(3, 4)
    print(f"Distance: {p1.distance_to(p2)}")

    for i, f in enumerate(fibonacci(10)):
        print(f"fib({i}) = {f}")

    data = DataProcessor([{"name": "alice", "age": 30}, {"name": "bob", "age": 25}])
    print(data.sort_by("age").to_json())

    primes = sieve_of_eratosthenes(100)
    print(f"Primes up to 100: {primes[:10]}...")

    text = "hello world hello python world"
    print(f"Word frequencies: {word_frequency(text)}")

    cache = CacheManager()
    cache.set("key1", "value1")
    print(f"Cache get: {cache.get('key1')}")


if __name__ == "__main__":
    main()
`

const pythonSingleFunc = `
"""
Lorem ipsum dolor sit amet consectetur adipiscing elit quisque faucibus ex
sapien vitae pellentesque sem placerat in id cursus mi pretium tellus duis
convallis tempus leo eu aenean sed diam urna tempor pulvinar vivamus fringilla
lacus nec metus bibendum egestas iaculis massa nisl malesuada lacinia integer
nunc posuere ut hendrerit semper vel class aptent taciti sociosqu ad litora
torquent per conubia nostra inceptos himenaeos orci varius natoque penatibus et
magnis dis parturient montes nascetur ridiculus mus donec rhoncus eros lobortis
nulla molestie mattis scelerisque maximus odio phasellus non purus est efficitur
laoreet mauris pharetra vestibulum fusce dictum risus blandit quis suspendisse
aliquet nisi sodales consequat magna ante condimentum neque at luctus nibh
finibus facilisis dapibus etiam interdum tortor ligula congue sollicitudin erat
viverra ac tincidunt nam porta elementum a enim euismod quam justo lectus
commodo augue arcu dignissim velit aliquam imperdiet mollis nullam volutpat
porttitor ullamcorper rutrum gravida cras eleifend turpis fames primis vulputate
ornare sagittis vehicula praesent dui felis venenatis ultrices proin libero
feugiat tristique accumsan maecenas potenti ultricies habitant morbi senectus
netus suscipit auctor curabitur facilisi cubilia curae hac habitasse platea
dictumst lorem ipsum dolor sit amet consectetur adipiscing elit quisque faucibus
ex sapien vitae pellentesque sem placerat in id cursus mi pretium tellus duis
convallis tempus leo eu aenean sed diam urna tempor pulvinar vivamus fringilla
lacus nec metus bibendum egestas iaculis massa nisl malesuada lacinia integer
nunc posuere ut hendrerit semper vel class aptent taciti sociosqu ad litora
torquent per conubia nostra inceptos himenaeos orci varius natoque penatibus et
magnis dis parturient montes nascetur ridiculus mus donec rhoncus eros lobortis
nulla molestie mattis scelerisque maximus eget fermentum odio phasellus non
purus est efficitur laoreet mauris pharetra vestibulum fusce dictum risus
blandit quis suspendisse aliquet nisi sodales consequat magna ante condimentum
neque at luctus nibh finibus facilisis dapibus etiam interdum tortor ligula
congue sollicitudin erat viverra ac tincidunt nam porta elementum a enim euismod
quam justo lectus commodo augue arcu dignissim velit aliquam imperdiet mollis
nullam volutpat porttitor ullamcorper rutrum gravida cras eleifend turpis fames
primis vulputate ornare sagittis vehicula praesent dui felis venenatis ultrices
proin libero feugiat tristique accumsan maecenas potenti ultricies habitant
morbi senectus netus suscipit auctor curabitur facilisi cubilia curae hac
habitasse platea dictumst lorem ipsum dolor sit amet consectetur adipiscing elit
quisque faucibus ex sapien vitae pellentesque sem placerat in id cursus mi
pretium tellus duis convallis tempus leo eu aenean sed diam urna tempor pulvinar
vivamus fringilla lacus nec metus bibendum egestas iaculis massa nisl malesuada
lacinia integer nunc posuere ut hend
"""
`

func TestChunkxParserPython(t *testing.T) {
	parser := &ChunkxParser{}
	doc, err := parser.Parse("test.py", pythonSample)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(doc.Chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	for i, chunk := range doc.Chunks {
		if chunk.ChunkIndex != i {
			t.Errorf("chunk[%d].ChunkIndex = %d, want %d", i, chunk.ChunkIndex, i)
		}
		if chunk.TextContent == "" {
			t.Errorf("chunk[%d].TextContent is empty", i)
		}
		if chunk.TotalChars <= 0 {
			t.Errorf("chunk[%d].TotalChars = %d, want > 0", i, chunk.TotalChars)
		}
		if chunk.LineStart <= 0 {
			t.Errorf("chunk[%d].LineStart = %d, want > 0", i, chunk.LineStart)
		}
		if chunk.LineEnd < chunk.LineStart {
			t.Errorf("chunk[%d].LineEnd (%d) < LineStart (%d)", i, chunk.LineEnd, chunk.LineStart)
		}
		if chunk.Metadata == "" {
			t.Errorf("chunk[%d].Metadata is empty", i)
		}
	}

	if doc.Metadata == "" {
		t.Error("doc.Metadata is empty")
	}
}

func TestChunkxParserLongPython(t *testing.T) {
	parser := &ChunkxParser{}
	doc, err := parser.Parse("test.py", pythonSampleLong)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(doc.Chunks) < 2 {
		t.Fatal("expected multiple chunks")
	}
}

func TestRawChunkxParserSingleLongNode(t *testing.T) {
	chunker := chunkx.NewChunker()
	chunks, err := chunker.Chunk(
		pythonSingleFunc,
		chunkx.WithLanguage(languages.Python),
		chunkx.WithMaxSize(DefaultChunkMaxSizeChar),
		chunkx.WithTokenCounter(&CharTokenizer{}),
	)
	if err != nil {
		t.Fatalf("chunkx.Chunk failed: %v", err)
	}

	if len(chunks) != 3 {
		t.Fatalf("Raw chunkx should not split a large node; got %d chunks", len(chunks))
	}
}

func TestChunkxParserSingleLongNode(t *testing.T) {
	parser := &ChunkxParser{}
	doc, err := parser.Parse("test.py", pythonSingleFunc)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	want := []project.ChunkRecord{
		{ChunkIndex: 0, TotalChars: 3, LineStart: 2, LineEnd: 2, SectionNum: 0},
		{ChunkIndex: 1, TotalChars: 1003, LineStart: 2, LineEnd: 16, SectionNum: 0},
		{ChunkIndex: 2, TotalChars: 1003, LineStart: 16, LineEnd: 29, SectionNum: 0},
		{ChunkIndex: 3, TotalChars: 1002, LineStart: 29, LineEnd: 42, SectionNum: 0},
		{ChunkIndex: 4, TotalChars: 3, LineStart: 42, LineEnd: 42, SectionNum: 0},
	}

	if len(doc.Chunks) != len(want) {
		t.Fatalf("got %d chunks, want %d", len(doc.Chunks), len(want))
	}

	for i, c := range doc.Chunks {
		w := want[i]
		if c.ChunkIndex != w.ChunkIndex ||
			c.TotalChars != w.TotalChars ||
			c.LineStart != w.LineStart ||
			c.LineEnd != w.LineEnd ||
			c.SectionNum != w.SectionNum {
			t.Errorf("chunk[%d] mismatch:\ngot  %+v\nwant %+v", i, c, w)
		}
	}
}
