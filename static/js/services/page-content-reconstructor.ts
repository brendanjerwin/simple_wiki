/**
 * Reconstructs the whole page text (front matter + markdown content)
 * from separate fields returned by the ReadPage gRPC response.
 *
 * The format is TOML front matter delimited by `+++` followed by
 * the markdown content body.
 */
export function reconstructWholePageText(
  frontMatterToml: string,
  contentMarkdown: string
): string {
  if (!frontMatterToml) {
    return contentMarkdown;
  }

  const normalizedToml = frontMatterToml.endsWith('\n')
    ? frontMatterToml
    : frontMatterToml + '\n';

  return `+++\n${normalizedToml}+++\n${contentMarkdown}`;
}
