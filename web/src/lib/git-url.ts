/**
 * Converts a git remote URL (SSH or HTTPS) to a browsable HTTP URL.
 * - git@github.com:user/repo.git → https://github.com/user/repo
 * - https://github.com/user/repo.git → https://github.com/user/repo
 */
export function gitUrlToHttp(url: string): string {
  // SSH format: git@host:user/repo.git
  const sshMatch = url.match(/^git@([^:]+):(.+)$/);
  if (sshMatch) {
    const [, host, path] = sshMatch;
    return `https://${host}/${path.replace(/\.git$/, "")}`;
  }

  // Already HTTPS — just strip trailing .git
  return url.replace(/\.git$/, "");
}
