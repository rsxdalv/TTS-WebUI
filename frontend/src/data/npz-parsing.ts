/*
 * Copyright 2023 aplbrain/npyjs
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/*
 * This file includes code adapted from the npyjs project,
 * which is licensed under the Apache License, Version 2.0.
 * The original code can be found at: https://github.com/aplbrain/npyjs/blob/master/index.js
 *
 * Modifications:
 * Added U1 support and fixed U1 -> u1 bug, added fortran comment and made a functional interface
 */

const dTypeMapping: Record<
  string,
  // "<u1" | "|u1" | "<u2" | "|i1" | "<i2" | "<u4" | "<i4" | "<u8" | "<i8" | "<f4" | "<f8" | "<U1",
  {
    name: string;
    size: number;
    arrayConstructor:
      | Uint8ArrayConstructor
      | Uint16ArrayConstructor
      | Int8ArrayConstructor
      | Int16ArrayConstructor
      | Int32ArrayConstructor
      | BigUint64ArrayConstructor
      | BigInt64ArrayConstructor
      | Float32ArrayConstructor
      | Float64ArrayConstructor
      | Uint32ArrayConstructor;
  }
> = {
  "<u1": {
    name: "uint8",
    size: 8,
    arrayConstructor: Uint8Array,
  },
  "|u1": {
    name: "uint8",
    size: 8,
    arrayConstructor: Uint8Array,
  },
  "<u2": {
    name: "uint16",
    size: 16,
    arrayConstructor: Uint16Array,
  },
  "|i1": {
    name: "int8",
    size: 8,
    arrayConstructor: Int8Array,
  },
  "<i2": {
    name: "int16",
    size: 16,
    arrayConstructor: Int16Array,
  },
  "<u4": {
    name: "uint32",
    size: 32,
    arrayConstructor: Int32Array,
  },
  "<i4": {
    name: "int32",
    size: 32,
    arrayConstructor: Int32Array,
  },
  "<u8": {
    name: "uint64",
    size: 64,
    arrayConstructor: BigUint64Array,
  },
  "<i8": {
    name: "int64",
    size: 64,
    arrayConstructor: BigInt64Array,
  },
  "<f4": {
    name: "float32",
    size: 32,
    arrayConstructor: Float32Array,
  },
  "<f8": {
    name: "float64",
    size: 64,
    arrayConstructor: Float64Array,
  },
  "<U1": {
    name: "<U1", // no way to know when to use ucs2 vs ucs4
    size: 32,
    arrayConstructor: Uint32Array,
  },
};

export const parseNpy = (arrayBufferContents: ArrayBuffer) => {
  // const version = arrayBufferContents.slice(6, 8); // Uint8-encoded
  const headerLength = new DataView(arrayBufferContents.slice(8, 10)).getUint8(
    0
  );
  const offsetBytes = 10 + headerLength;

  const hcontents = new TextDecoder("utf-8").decode(
    new Uint8Array(arrayBufferContents.slice(10, 10 + headerLength))
  );
  const header = JSON.parse(
    hcontents
      // .toLowerCase() // True -> true
      .replace(/True/g, "true")
      .replace(/False/g, "false")
      .replace(/'/g, '"')
      .replace("(", "[")
      .replace(/,*\),*/g, "]")
  );
  const shape = header.shape;
  const dtype = dTypeMapping[header.descr];
  if (!dtype) {
    console.log(header);
    console.log(hcontents);
    throw new Error(`Unknown dtype: ${header.descr}`);
  }
  const nums = new dtype["arrayConstructor"](arrayBufferContents, offsetBytes);

  // if fortran_order:
  //     array.shape = shape[::-1]
  //     array = array.transpose()

  return {
    dtype: dtype.name,
    data: nums,
    shape,
    fortranOrder: header.fortran_order,
  };
};
