class UserService {
    async findUser(id: string): Promise<User> {
        const user = await db.users.findOne({ id });
        if (!user) throw new NotFoundError();
        return user;
    }

    async deleteUser(id: string): Promise<void> {
        await db.users.delete({ id });
    }
}
