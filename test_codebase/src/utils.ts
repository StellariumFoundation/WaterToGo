export interface User {
    id: number;
    name: string;
    email: string;
}

export function formatUser(user: User): string {
    return `${user.name} <${user.email}>`;
}

export function calculateScore(points: number, bonus: number): number {
    return points * (1 + bonus / 100);
}
