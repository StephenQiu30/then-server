export function passwordLengthValid(password: string): boolean {
  const bytes = new TextEncoder().encode(password).length
  return bytes >= 12 && bytes <= 72
}

export function registrationErrors(values: {
  displayName: string
  email: string
  password: string
  confirmation: string
}): Partial<
  Record<'displayName' | 'email' | 'password' | 'confirmation', string>
> {
  const errors: Partial<
    Record<'displayName' | 'email' | 'password' | 'confirmation', string>
  > = {}
  if (
    !values.displayName.trim() ||
    Array.from(values.displayName.trim()).length > 80
  ) {
    errors.displayName = '请输入不超过 80 个字符的显示名称。'
  }
  if (!values.email.trim()) errors.email = '请输入邮箱。'
  if (!passwordLengthValid(values.password))
    errors.password = '密码需为 12–72 个 UTF-8 字节。'
  if (values.password !== values.confirmation)
    errors.confirmation = '两次输入的密码不一致。'
  return errors
}
