import { z } from "zod";

const runeLength = (text: string) => [...text].length;
const validPassword = (value: string, minimum: number) => {
  const size = runeLength(value);
  return (
    size >= minimum &&
    size <= 128 &&
    new TextEncoder().encode(value).length <= 512
  );
};

// The backend normalizes ASCII addresses and rejects display names. Let it
// decide uncommon address syntax that browsers may normalize differently.
const email = z
  .string()
  .trim()
  .toLowerCase()
  .refine(
    (value) =>
      value.length <= 254 &&
      /^[!-?A-~]+@[!-?A-~]+$/.test(value) &&
      !/[<>,"]/.test(value) &&
      !value.includes(".."),
    "Enter a valid email address.",
  );

export const signInSchema = z.object({
  name: z.string(),
  email,
  password: z
    .string()
    .refine(
      (value) => validPassword(value, 1),
      "Enter your password (up to 128 characters).",
    ),
});

export const registerSchema = signInSchema.extend({
  name: z
    .string()
    .trim()
    .refine(
      (value) =>
        runeLength(value) >= 1 &&
        runeLength(value) <= 80 &&
        !value.includes("\0"),
      "Name must contain 1–80 characters.",
    ),
  password: z
    .string()
    .refine(
      (value) => validPassword(value, 12),
      "Password must contain 12–128 characters and at most 512 bytes.",
    ),
});

export type AuthFields = z.input<typeof signInSchema>;
