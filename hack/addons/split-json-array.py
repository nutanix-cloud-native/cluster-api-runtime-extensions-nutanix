#!/usr/bin/env python3
# Copyright 2024 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

"""Split a JSON array into multiple files, each staying under a byte size limit.

Uses a greedy first-fit-decreasing bin-packing algorithm: elements are sorted
by serialized size (largest first), then each element is placed into the first
bin that has room. This minimizes the number of output files.
"""

import argparse
import json
import sys
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, help="Path to input JSON array file")
    parser.add_argument("--output-dir", required=True, help="Directory for output chunk files")
    parser.add_argument("--output-prefix", required=True, help="Filename prefix for chunks")
    parser.add_argument(
        "--max-bytes",
        type=int,
        default=900_000,
        help="Maximum bytes per output file (default: 900000)",
    )
    args = parser.parse_args()

    with open(args.input) as f:
        data = json.load(f)

    if not isinstance(data, list):
        print("Error: input JSON must be an array", file=sys.stderr)
        sys.exit(1)

    elements = [obj for obj in data if obj is not None]

    sized = [(elem, len(json.dumps(elem, separators=(",", ":")))) for elem in elements]
    sized.sort(key=lambda x: x[1], reverse=True)

    # JSON array overhead: opening '[', closing ']', and ',' between elements.
    # Per-element overhead is 1 byte for the comma separator (except the last).
    ARRAY_OVERHEAD = 2  # '[]'

    buckets: list[list] = []
    bucket_sizes: list[int] = []

    for elem, size in sized:
        placed = False
        for i in range(len(buckets)):
            separator_cost = 1 if buckets[i] else 0
            if bucket_sizes[i] + size + separator_cost <= args.max_bytes:
                buckets[i].append(elem)
                bucket_sizes[i] += size + separator_cost
                placed = True
                break
        if not placed:
            buckets.append([elem])
            bucket_sizes.append(ARRAY_OVERHEAD + size)

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    for i, bucket in enumerate(buckets):
        output_path = output_dir / f"{args.output_prefix}-{i}.json"
        with open(output_path, "w") as f:
            json.dump(bucket, f, separators=(",", ":"))
        print(f"Wrote {output_path} ({len(bucket)} elements, {output_path.stat().st_size} bytes)")


if __name__ == "__main__":
    main()
