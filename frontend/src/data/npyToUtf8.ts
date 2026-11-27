import { decode } from "iconv-lite";

export const npyToUtf8 = (item) => {
  const { data, shape } = item;
  const string = decode(
    Buffer.from(data.buffer, data.byteOffset, data.byteLength),
    "utf32le"
  );
  if (string.length !== shape[0])
    throw new Error(
      `Expected string length ${shape[0]}, but got ${string.length}`
    );
  return string;
};

if ("test" === process.argv[2]) {
  const npyList = {
    dtype: "<U1",
    data: Uint32Array.from([65, 66, 28450]),
    shape: [3],
    fortranOrder: false,
  };
  const y = npyToUtf8(npyList);
  if (y !== "AB漢") throw new Error("npyToUtf8 failed");
  console.log("npyToUtf8 passed");
  console.log(y);
}
