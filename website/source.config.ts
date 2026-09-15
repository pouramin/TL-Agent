import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { metaSchema, pageSchema } from 'fumadocs-core/source/schema';
import { z } from 'zod';

export const docs = defineDocs({
  dir: 'content/docs',
  docs: {
    schema: pageSchema.extend({
      description: z.string().min(1, 'Every public documentation page needs a description.'),
    }),
  },
  meta: {
    schema: metaSchema,
  },
});

export default defineConfig({});
